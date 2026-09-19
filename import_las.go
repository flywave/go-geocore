package geocore

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/flywave/go-las"
)

// LASProjectConfig 控制 LAS → Project 的映射。
type LASProjectConfig struct {
	// 井口坐标覆盖。LAS 头部没有坐标字段（或需要纠偏）时由调用方给出；
	// HasCollar 为 true 时忽略文件中的坐标。
	X, Y, Elevation float64
	HasCollar       bool
	// WellID 覆盖井号；缺省依次取 UWI / API / WELL / 文件名
	WellID string
	// LithologyCurves 岩性曲线助记符，按顺序取第一个命中的曲线。
	// 曲线值连续相同的深度段合并为一个地层单元（LITH/LITHO/LITHOLOGY/FACIES 等编码曲线）。
	LithologyCurves []string
	// LithologyNames 岩性编码 → 名称，缺省直接用编码本身
	LithologyNames map[float64]string
}

var DefaultLASProjectConfig = &LASProjectConfig{
	LithologyCurves: []string{"LITH", "LITHO", "LITHOLOGY", "LITHCODE", "LITH_CODE", "FACIES", "ROCK", "UNIT"},
}

// ImportLAS 读取一口井的测井曲线（GR/RT/DT/DEN...）。
//
// 井口坐标与井口高程取自 LAS 头部（X/Y/LATI/LONGI/ELEV/KB/GL 等助记符）；
// 头部缺失时坐标保持 0，由调用方按需补全（见 ImportLASProjectWithConfig）。
func ImportLAS(path string) (*Well, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	l, err := las.Parse(f)
	if err != nil {
		return nil, err
	}

	w := &Well{ID: l.Well.Well, Logs: make(map[string]*LogCurve), Meta: make(map[string]string)}

	depth := l.DepthColumn()
	for _, c := range l.Curves {
		if c.Mnemonic == "DEPT" || c.Mnemonic == "DEPTH" {
			continue
		}
		curve := &LogCurve{
			Mnemonic: c.Mnemonic,
			Unit:     c.Unit,
			Points:   make([]LogSample, len(c.Data)),
		}
		for i, v := range c.Data {
			if i < len(depth) {
				curve.Points[i] = LogSample{Depth: depth[i], Value: v}
			}
		}
		w.Logs[c.Mnemonic] = curve
	}
	if l.Well.Company != "" {
		w.Meta["company"] = l.Well.Company
	}
	if l.Well.Field != "" {
		w.Meta["field"] = l.Well.Field
	}
	if l.Well.Well != "" {
		w.Meta["well"] = l.Well.Well
	}

	// 井口坐标只能从头部原始字段取：go-las 只保留固定的几个井信息助记符，
	// X/Y/ELEV/LATI/LONGI 会被丢弃（此前实现退而用 StartIndex 当高程，
	// 那是起始深度不是高程，会把井口埋到地下）。
	header, err := lasHeaderFields(path)
	if err != nil {
		return nil, err
	}
	collar, ok := lasCollarFromHeader(header)
	if ok {
		w.X, w.Y, w.Elevation = collar.X, collar.Y, collar.Elevation
		w.Meta["collar_source"] = collar.source
	}
	for k, v := range header {
		switch k {
		case "API", "UWI", "LOC", "PROV", "CNTY", "CTRY", "LIC", "DATE", "SRVC":
			if v != "" {
				w.Meta[strings.ToLower(k)] = v
			}
		}
	}
	return w, nil
}

// ImportLASProject 把单个 LAS 文件转成 Project（一口井：井口 + 测井曲线 + 可选地层）。
//
// 井口坐标必须能从文件中解析出来，否则直接报错——不臆造坐标。
// 地层由岩性编码曲线（LITH/FACIES 等）按连续相同值分段得到；
// 文件中没有这类曲线时只产出井口与曲线，地层留空（不静默造层）。
func ImportLASProject(path string) (*Project, error) {
	return ImportLASProjectWithConfig(path, nil)
}

func ImportLASProjectWithConfig(path string, cfg *LASProjectConfig) (*Project, error) {
	if cfg == nil {
		cfg = DefaultLASProjectConfig
	}
	lithCurves := cfg.LithologyCurves
	if len(lithCurves) == 0 {
		// 只覆盖了坐标等个别字段的配置不应丢掉默认的岩性曲线识别
		lithCurves = DefaultLASProjectConfig.LithologyCurves
	}

	w, err := ImportLAS(path)
	if err != nil {
		return nil, fmt.Errorf("read LAS %q: %w", path, err)
	}

	if cfg.HasCollar {
		w.X, w.Y, w.Elevation = cfg.X, cfg.Y, cfg.Elevation
		w.Meta["collar_source"] = "caller"
	} else if w.X == 0 && w.Y == 0 {
		return nil, fmt.Errorf(
			"LAS %q 缺少井口坐标：头部没有 X/Y（或 LONGI/LATI）字段，无法确定井位。"+
				"请在文件中补 X./Y. 字段，或用 LASProjectConfig{HasCollar:true, X:.., Y:.., Elevation:..} 显式给出", path)
	}

	if cfg.WellID != "" {
		w.ID = cfg.WellID
	} else {
		w.ID = lasWellID(w, path)
	}

	if curve, ok := pickLithologyCurve(w, lithCurves); ok {
		w.Strata = strataFromLithologyCurve(curve, w.Elevation, cfg.LithologyNames)
		w.Meta["lithology_curve"] = curve.Mnemonic
		if n := len(w.Strata); n > 0 {
			w.Depth = w.Strata[n-1].BaseMD
		}
	}

	proj := NewProject(filepath.Base(path))
	proj.Meta["source"] = path
	proj.Meta["format"] = "LAS"
	proj.Wells = append(proj.Wells, *w)
	return proj, nil
}

// lasWellID 选择井号：UWI/API 最稳定，其次 WELL 名，最后退到文件名
func lasWellID(w *Well, path string) string {
	for _, k := range []string{"uwi", "api", "well"} {
		if v := strings.TrimSpace(w.Meta[k]); v != "" {
			return v
		}
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// pickLithologyCurve 按助记符优先级挑选岩性曲线
func pickLithologyCurve(w *Well, mnemonics []string) (*LogCurve, bool) {
	for _, m := range mnemonics {
		if c, ok := w.Logs[strings.ToUpper(m)]; ok && len(c.Points) > 0 {
			return c, true
		}
	}
	return nil, false
}

// strataFromLithologyCurve 把岩性编码曲线切成地层单元：
// 深度连续且编码相同的一段合成一个单元，深度方向（递增/递减）无关。
//
// 一个采样点代表 [depth, depth+step) 的区间，因此单元底界要外推一个采样步长，
// 否则最后一层会缺一个采样间隔的厚度。
func strataFromLithologyCurve(curve *LogCurve, elev float64, names map[float64]string) []StratumInterval {
	step := medianDepthStep(curve.Points)

	var out []StratumInterval
	var start, last float64
	var code float64
	inRun := false

	flush := func() {
		if !inRun {
			return
		}
		top, base := start, last
		if top > base {
			top, base = base, top
		}
		base += step
		iv := StratumInterval{
			TopMD:     top,
			BaseMD:    base,
			Thickness: base - top,
			TopElev:   elev - top,
			BaseElev:  elev - base,
		}
		if name, ok := names[code]; ok && name != "" {
			iv.Lithology = name
		} else {
			iv.Lithology = formatFloatTrim(code)
		}
		iv.ID = "L" + iv.Lithology
		out = append(out, iv)
		inRun = false
	}

	for _, p := range curve.Points {
		if math.IsNaN(p.Value) {
			// NULL 值不构成地层，作为断点
			flush()
			continue
		}
		if !inRun {
			start, last, code, inRun = p.Depth, p.Depth, p.Value, true
			continue
		}
		if p.Value != code {
			flush()
			start, last, code, inRun = p.Depth, p.Depth, p.Value, true
			continue
		}
		last = p.Depth
	}
	flush()

	// 统一从浅到深排序，Index 0 = 最上层
	sort.SliceStable(out, func(i, j int) bool { return out[i].TopMD < out[j].TopMD })
	for i := range out {
		out[i].Index = i
	}
	return out
}

// medianDepthStep 采样步长的中位数（取绝对值），忽略零间隔
func medianDepthStep(points []LogSample) float64 {
	var steps []float64
	for i := 1; i < len(points); i++ {
		d := math.Abs(points[i].Depth - points[i-1].Depth)
		if d > 0 {
			steps = append(steps, d)
		}
	}
	if len(steps) == 0 {
		return 0
	}
	sort.Float64s(steps)
	return steps[len(steps)/2]
}

// ─── LAS 头部原始字段 ───────────────────────────────────────

// lasHeaderFields 扫描 LAS 头部（~W 井信息段、~P 参数段与无段标记的历史写法），
// 返回助记符 → 值。只读数据段（~A/~ASCII）之前的行。
func lasHeaderFields(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string)
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r\n\t "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "~") {
			// 数据段开始，头部结束
			if isLASDataSection(line) {
				break
			}
			continue
		}
		mnem, val, ok := splitLASField(line)
		if !ok {
			continue
		}
		if _, seen := fields[mnem]; !seen {
			fields[mnem] = val
		}
	}
	return fields, nil
}

func isLASDataSection(line string) bool {
	upper := strings.ToUpper(line)
	return strings.HasPrefix(upper, "~A") || strings.HasPrefix(upper, "~ASCII")
}

// splitLASField 解析 "MNEM.UNIT  VALUE : DESCRIPTION"，
// 返回大写助记符与 VALUE 中第一个可解析为数字的记号（坐标/高程是数值字段）。
func splitLASField(line string) (string, string, bool) {
	dot := strings.Index(line, ".")
	if dot <= 0 || dot > 10 {
		return "", "", false
	}
	mnem := strings.ToUpper(strings.TrimSpace(line[:dot]))
	if mnem == "" || strings.ContainsAny(mnem, " \t") {
		return "", "", false
	}
	rest := line[dot+1:]
	if colon := strings.Index(rest, ":"); colon >= 0 {
		rest = rest[:colon]
	}
	return mnem, strings.TrimSpace(rest), true
}

type lasCollar struct {
	X, Y, Elevation float64
	source          string
}

// lasCollarFromHeader 从头部字段解析井口坐标。
// 支持 X/Y 平面坐标，也支持 LONGI/LATI 经纬度（分别作为 X/Y）；
// X 与 Y 必须同时具备，只有一半的坐标视为缺失（否则会造出一口假井位）。
func lasCollarFromHeader(fields map[string]string) (lasCollar, bool) {
	c := lasCollar{source: "header"}

	x, okX := lasHeaderNumber(fields, "X", "XCOORD", "X_COORD", "EAST", "EASTING")
	if !okX {
		x, okX = lasHeaderNumber(fields, "LONGI", "LONG", "LON")
	}
	y, okY := lasHeaderNumber(fields, "Y", "YCOORD", "Y_COORD", "NORTH", "NORTHING")
	if !okY {
		y, okY = lasHeaderNumber(fields, "LATI", "LAT")
	}
	if !okX || !okY {
		return lasCollar{}, false
	}
	c.X, c.Y = x, y

	if v, ok := lasHeaderNumber(fields, "ELEV", "ELEVATION", "KB", "GL", "SRFELEV"); ok {
		c.Elevation = v
	}
	return c, true
}

// lasHeaderNumber 按助记符顺序取第一个能解析为数值的字段
func lasHeaderNumber(fields map[string]string, mnemonics ...string) (float64, bool) {
	for _, m := range mnemonics {
		raw, ok := fields[m]
		if !ok {
			continue
		}
		for _, tok := range strings.Fields(raw) {
			if v, err := strconv.ParseFloat(strings.Trim(tok, ","), 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

func formatFloatTrim(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}
