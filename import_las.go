package geocore

import (
	"os"

	"github.com/flywave/go-las"
)

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

	w := &Well{
		ID:   l.Well.Well,
		Logs: make(map[string]*LogCurve),
		Meta: make(map[string]string),
	}
	w.Location = [3]float64{0, 0, l.Well.StartIndex}

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
	if l.Well.Location != "" {
		w.Meta["location"] = l.Well.Location
	}

	return w, nil
}
