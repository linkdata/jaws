package jaws

import (
	"fmt"
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
	return fmt.Sprintf("%d %d %d %s", clk.X, clk.Y, clk.keyState(), clk.Name)
}

func parseClickData(val string) (clk Click, after string, ok bool) {
	var clickPart string
	clickPart, after, _ = strings.Cut(val, "\t")
	var n int
	var kstate int
	ok = true
	for field := range strings.FieldsSeq(clickPart) {
		if ok {
			switch n {
			case 0:
				clk.X, ok = runAtoi(field)
			case 1:
				clk.Y, ok = runAtoi(field)
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

func runAtoi(v string) (n int, ok bool) {
	var err error
	n, err = strconv.Atoi(v)
	ok = err == nil
	return
}
