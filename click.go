package jaws

import (
	"math"
	"strconv"
	"strings"
)

// Click identifies a browser click-like event, pointer location and modifier state.
//
// X and Y are browser viewport CSS-pixel coordinates for regular HTML events.
// For SVG events they are converted by the browser to the nearest owning SVG
// viewport's user coordinate system, including viewBox, CSS scaling and SVG
// transforms.
type Click struct {
	Name    string
	X       float64
	Y       float64
	Shift   bool
	Control bool
	Alt     bool
}

const (
	clickKeyShift = (1 << iota)
	clickKeyControl
	clickKeyAlt
)

func (clk Click) keyState() (state int) {
	if clk.Shift {
		state |= clickKeyShift
	}
	if clk.Control {
		state |= clickKeyControl
	}
	if clk.Alt {
		state |= clickKeyAlt
	}
	return
}

func (clk *Click) setKeyState(state int) {
	clk.Shift = (state & clickKeyShift) != 0
	clk.Control = (state & clickKeyControl) != 0
	clk.Alt = (state & clickKeyAlt) != 0
}

// String formats clk for the JaWS wire protocol.
func (clk Click) String() string {
	return strconv.FormatFloat(clk.X, 'g', -1, 64) + " " +
		strconv.FormatFloat(clk.Y, 'g', -1, 64) + " " +
		strconv.Itoa(clk.keyState()) + " " + clk.Name
}

func parseClickData(value string) (clk Click, after string, ok bool) {
	var clickPart string
	clickPart, after, _ = strings.Cut(value, "\t")
	var n int
	var kstate int
	ok = true
	for field := range strings.FieldsSeq(clickPart) {
		if ok {
			switch n {
			case 0:
				clk.X, ok = runAtof(field)
			case 1:
				clk.Y, ok = runAtof(field)
			case 2:
				kstate, ok = runAtoi(field)
				clk.setKeyState(kstate)
			case 3:
				clk.Name = field
			default:
				clk.Name += " " + field
			}
			n++
		}
	}
	ok = ok && n >= 3
	return
}

func runAtof(value string) (f float64, ok bool) {
	var err error
	f, err = strconv.ParseFloat(value, 64)
	ok = err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
	return
}

func runAtoi(value string) (n int, ok bool) {
	var err error
	n, err = strconv.Atoi(value)
	ok = err == nil
	return
}
