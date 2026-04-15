package jaws

import (
	"strconv"
	"strings"
)

// Click identifies a browser click-like event, pointer location and modifier state.
type Click struct {
	Name    string
	X       int
	Y       int
	Shift   bool
	Control bool
	Alt     bool
}

func (clk Click) String() string {
	var b []byte
	b = append(b, clk.Name...)
	b = append(b, '\t')
	b = strconv.AppendInt(b, int64(clk.X), 10)
	b = append(b, '\t')
	b = strconv.AppendInt(b, int64(clk.Y), 10)
	b = append(b, '\t')
	b = strconv.AppendBool(b, clk.Shift)
	b = append(b, '\t')
	b = strconv.AppendBool(b, clk.Control)
	b = append(b, '\t')
	b = strconv.AppendBool(b, clk.Alt)
	return string(b)
}

func parseClickData(val string) (clk Click, after string, ok bool) {
	parts := strings.Split(val, "\t")
	if len(parts) < 3 {
		return
	}
	clk.Name = parts[0]
	if clk.X, ok = parseClickCoord(parts[1]); !ok {
		return
	}
	if clk.Y, ok = parseClickCoord(parts[2]); !ok {
		return
	}
	idx := 3
	if idx < len(parts) {
		if v, berr := strconv.ParseBool(parts[idx]); berr == nil {
			clk.Shift = v
			idx++
			if idx < len(parts) {
				if v, berr = strconv.ParseBool(parts[idx]); berr == nil {
					clk.Control = v
					idx++
					if idx < len(parts) {
						if v, berr = strconv.ParseBool(parts[idx]); berr == nil {
							clk.Alt = v
							idx++
						}
					}
				}
			}
		}
	}
	after = strings.Join(parts[idx:], "\t")
	ok = true
	return
}

func parseClickCoord(v string) (n int, ok bool) {
	var err error
	n, err = strconv.Atoi(v)
	ok = err == nil
	return
}
