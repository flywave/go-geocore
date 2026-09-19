package geocore

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

type BoreholeCSVConfig struct {
	CollarPath string
	SurveyPath string // optional
	LithPath   string // optional
	IDCol      string
	XCol       string
	YCol       string
	ZCol       string
	MDCol      string
	AzCol      string
	DipCol     string
	TopCol     string
	BaseCol    string
	LithCol    string
	// StratumIDCol 可选的地层编号列（如 "unit"/"formation"）。
	// 缺省 "unit"；该列不存在时按井内顺序生成 "S0","S1",...
	StratumIDCol string
	HasHeader    bool
}

var DefaultBoreholeConfig = &BoreholeCSVConfig{
	IDCol: "id", XCol: "x", YCol: "y", ZCol: "z",
	MDCol: "md", AzCol: "az", DipCol: "dip",
	TopCol: "top", BaseCol: "base", LithCol: "lith",
	StratumIDCol: "unit",
	HasHeader:    true,
}

func ImportBoreholeCSV(collarPath, surveyPath, lithPath string) ([]*Well, error) {
	return ImportBoreholeCSVWithConfig(collarPath, surveyPath, lithPath, DefaultBoreholeConfig)
}

func ImportBoreholeCSVWithConfig(collarPath, surveyPath, lithPath string, cfg *BoreholeCSVConfig) ([]*Well, error) {
	if cfg == nil {
		cfg = DefaultBoreholeConfig
	}

	collars, err := readCollarsCSV(collarPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("read collars: %v", err)
	}

	surveys := make(map[string][]SurveyPoint)
	if surveyPath != "" {
		surveys, err = readSurveyCSV(surveyPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("read survey: %v", err)
		}
	}

	liths := make(map[string][]StratumInterval)
	if lithPath != "" {
		liths, err = readLithologyCSV(lithPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("read lithology: %v", err)
		}
	}

	wells := make([]*Well, 0, len(collars))
	for _, collar := range collars {
		id := collar.id
		w := &Well{ID: id, X: collar.x, Y: collar.y, Elevation: collar.z, Logs: make(map[string]*LogCurve), Meta: make(map[string]string)}
		if pts, ok := surveys[id]; ok {
			w.Surveys = pts
		}
		if ss, ok := liths[id]; ok {
			// 岩性 CSV 的 top/base 是测量深度（MD），井口高程来自 collars.z。
			// 早期实现把 Elevation 覆盖成 ss[0].TopElev（从未计算，恒为 0），
			// 导致所有地层高程塌到 0；这里改为由井口高程换算绝对高程。
			for i := range ss {
				ss[i].Index = i
				ss[i].TopElev = collar.z - ss[i].TopMD
				ss[i].BaseElev = collar.z - ss[i].BaseMD
				if ss[i].Thickness == 0 {
					ss[i].Thickness = ss[i].BaseMD - ss[i].TopMD
				}
			}
			w.Strata = ss
			if n := len(ss); n > 0 {
				w.Depth = ss[n-1].BaseMD
			}
		}
		wells = append(wells, w)
	}

	return wells, nil
}

type collarRow struct {
	id      string
	x, y, z float64
}

func readCollarsCSV(path string, cfg *BoreholeCSVConfig) ([]collarRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("empty collars CSV")
	}

	colMap := make(map[string]int)
	start := 0
	if cfg.HasHeader {
		for i, h := range rows[0] {
			colMap[h] = i
		}
		start = 1
	} else {
		colMap[cfg.IDCol] = 0
		colMap[cfg.XCol] = 1
		colMap[cfg.YCol] = 2
		colMap[cfg.ZCol] = 3
	}

	cols := func(name string) int {
		if idx, ok := colMap[name]; ok {
			return idx
		}
		return -1
	}

	idIdx := cols(cfg.IDCol)
	xIdx := cols(cfg.XCol)
	yIdx := cols(cfg.YCol)
	zIdx := cols(cfg.ZCol)

	if idIdx < 0 || xIdx < 0 || yIdx < 0 || zIdx < 0 {
		return nil, fmt.Errorf("collars CSV missing required columns (need id, x, y, z), got headers: %v", colMap)
	}

	var result []collarRow
	for _, row := range rows[start:] {
		if len(row) <= maxIdx(idIdx, xIdx, yIdx, zIdx) {
			continue
		}
		x, _ := strconv.ParseFloat(row[xIdx], 64)
		y, _ := strconv.ParseFloat(row[yIdx], 64)
		z, _ := strconv.ParseFloat(row[zIdx], 64)
		result = append(result, collarRow{id: row[idIdx], x: x, y: y, z: z})
	}
	return result, nil
}

func readSurveyCSV(path string, cfg *BoreholeCSVConfig) (map[string][]SurveyPoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("empty survey CSV")
	}

	colMap := make(map[string]int)
	start := 0
	if cfg.HasHeader {
		for i, h := range rows[0] {
			colMap[h] = i
		}
		start = 1
	} else {
		colMap[cfg.IDCol] = 0
		colMap[cfg.MDCol] = 1
	}

	idIdx := colMap[cfg.IDCol]
	mdIdx := colMap[cfg.MDCol]
	azIdx := -1
	dipIdx := -1
	if idx, ok := colMap[cfg.AzCol]; ok {
		azIdx = idx
	}
	if idx, ok := colMap[cfg.DipCol]; ok {
		dipIdx = idx
	}

	result := make(map[string][]SurveyPoint)
	for _, row := range rows[start:] {
		if len(row) <= idIdx || len(row) <= mdIdx {
			continue
		}
		md, _ := strconv.ParseFloat(row[mdIdx], 64)
		sp := SurveyPoint{MD: md}
		if azIdx >= 0 && azIdx < len(row) {
			sp.Azimuth, _ = strconv.ParseFloat(row[azIdx], 64)
		}
		if dipIdx >= 0 && dipIdx < len(row) {
			sp.Inclination, _ = strconv.ParseFloat(row[dipIdx], 64)
		}
		if len(row) > maxIdx(idIdx, mdIdx)+2 {
			sp.X, _ = strconv.ParseFloat(row[mdIdx+1], 64)
			sp.Y, _ = strconv.ParseFloat(row[mdIdx+2], 64)
			if len(row) > mdIdx+3 {
				sp.Z, _ = strconv.ParseFloat(row[mdIdx+3], 64)
			}
		}
		id := row[idIdx]
		result[id] = append(result[id], sp)
	}
	return result, nil
}

func readLithologyCSV(path string, cfg *BoreholeCSVConfig) (map[string][]StratumInterval, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("empty lithology CSV")
	}

	colMap := make(map[string]int)
	start := 0
	if cfg.HasHeader {
		for i, h := range rows[0] {
			colMap[h] = i
		}
		start = 1
	}

	idIdx := colMap[cfg.IDCol]
	topIdx := colMap[cfg.TopCol]
	baseIdx := colMap[cfg.BaseCol]
	lithIdx := -1
	if idx, ok := colMap[cfg.LithCol]; ok {
		lithIdx = idx
	}
	unitIdx := -1
	if cfg.StratumIDCol != "" {
		if idx, ok := colMap[cfg.StratumIDCol]; ok {
			unitIdx = idx
		}
	}

	result := make(map[string][]StratumInterval)
	for _, row := range rows[start:] {
		if len(row) <= maxIdx(idIdx, topIdx, baseIdx) {
			continue
		}
		top, _ := strconv.ParseFloat(row[topIdx], 64)
		base, _ := strconv.ParseFloat(row[baseIdx], 64)
		if base <= top {
			continue
		}
		s := StratumInterval{
			TopMD:     top,
			BaseMD:    base,
			Thickness: base - top,
		}
		if lithIdx >= 0 && lithIdx < len(row) {
			s.Lithology = row[lithIdx]
		}
		id := row[idIdx]
		// 地层编号：显式列优先，否则按井内顺序生成 S0/S1/...
		// go-geology 用地层 ID 作为地质单元键（地震层位也按 ID 匹配），
		// 空 ID 会让所有地层塌成同一单元。
		if unitIdx >= 0 && unitIdx < len(row) && row[unitIdx] != "" {
			s.ID = row[unitIdx]
		} else {
			s.ID = fmt.Sprintf("S%d", len(result[id]))
		}
		result[id] = append(result[id], s)
	}
	return result, nil
}

func maxIdx(indices ...int) int {
	m := 0
	for _, v := range indices {
		if v > m {
			m = v
		}
	}
	return m
}
