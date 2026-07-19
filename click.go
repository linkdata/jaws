package jaws

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Click identifies a browser click-like event, pointer location and modifier state.
type Click struct {
	// Name is the event target name. Parsing off the wire normalizes it: leading
	// and trailing whitespace is trimmed and internal whitespace runs collapse to a
	// single space, so it does not round-trip losslessly through [Click.String].
	Name    string
	X       float64 // X is the browser clientX coordinate in CSS pixels.
	Y       float64 // Y is the browser clientY coordinate in CSS pixels.
	Shift   bool    // Shift reports whether the Shift key was held during the event.
	Control bool    // Control reports whether the Control key was held during the event.
	Alt     bool    // Alt reports whether the Alt key was held during the event.
}

const (
	clickKeyShift = (1 << iota)
	clickKeyControl
	clickKeyAlt
	clickKeyMask = clickKeyShift | clickKeyControl | clickKeyAlt
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

// setKeyState sets the Shift, Control and Alt fields from the bitmask produced by
// keyState, used when parsing an incoming click off the wire.
func (clk *Click) setKeyState(state int) {
	clk.Shift = (state & clickKeyShift) != 0
	clk.Control = (state & clickKeyControl) != 0
	clk.Alt = (state & clickKeyAlt) != 0
}

// String formats clk for the JaWS wire protocol.
//
// It is not a lossless inverse of parsing: a [Click.Name] with leading, trailing
// or repeated internal whitespace is normalized when parsed back (see the Name
// field). The production wire direction is browser-to-server (parse only).
func (clk Click) String() string {
	return fmt.Sprintf("%s %s %d %s", runFormatFloat(clk.X), runFormatFloat(clk.Y), clk.keyState(), clk.Name)
}

func parseClickData(value string) (clk Click, after string, ok bool) {
	var clickPart string
	clickPart, after, _ = strings.Cut(value, "\t")
	var n int
	var kstate int
	var name strings.Builder
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
				if ok {
					ok = kstate >= 0 && kstate&^clickKeyMask == 0
				}
				if ok {
					clk.setKeyState(kstate)
				}
			case 3:
				// First name token: assign directly so the common single-token click
				// stays allocation-free.
				clk.Name = field
			default:
				// Second or later name token: accumulate with a strings.Builder, seeded
				// once with the first token and joined by single spaces, so the name stays
				// O(total length) rather than the O(n^2) of naive per-token string
				// concatenation. The click frame is untrusted browser input sized up to the
				// WebSocket read limit.
				if name.Len() == 0 {
					name.Grow(len(clk.Name) + 1 + len(field))
					name.WriteString(clk.Name)
				}
				name.WriteByte(' ')
				name.WriteString(field)
			}
			n++
		}
	}
	if name.Len() > 0 {
		clk.Name = name.String()
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
