package jaws

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Click identifies a browser click-like event, pointer location and modifier state.
type Click struct {
	Name    string
	X       float64 // X is the browser clientX coordinate in CSS pixels.
	Y       float64 // Y is the browser clientY coordinate in CSS pixels.
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
	return fmt.Sprintf("%s %s %d %s", runFormatFloat(clk.X), runFormatFloat(clk.Y), clk.keyState(), clk.Name)
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

func runFormatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func runAtof(value string) (n float64, ok bool) {
	var err error
	n, err = strconv.ParseFloat(value, 64)
	ok = err == nil && !math.IsInf(n, 0) && !math.IsNaN(n)
	return
}

func runAtoi(value string) (n int, ok bool) {
	var err error
	n, err = strconv.Atoi(value)
	ok = err == nil
	return
}
