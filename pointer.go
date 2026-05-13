package jaws

import (
	"strconv"
	"strings"
)

// PointerKind identifies which browser pointer event produced a [Pointer].
type PointerKind uint8

const (
	// PointerDown is sent for browser pointerdown events.
	PointerDown PointerKind = iota + 1
	// PointerMove is sent for browser pointermove events while a button is held.
	PointerMove
	// PointerUp is sent for browser pointerup events.
	PointerUp
	// PointerCancel is sent for browser pointercancel events.
	PointerCancel
)

const (
	// PointerButtonPrimary is the browser buttons bit for the primary button.
	PointerButtonPrimary = 1 << iota
	// PointerButtonSecondary is the browser buttons bit for the secondary button.
	PointerButtonSecondary
	// PointerButtonAuxiliary is the browser buttons bit for the auxiliary button.
	PointerButtonAuxiliary
	// PointerButtonBack is the browser buttons bit for the browser back button.
	PointerButtonBack
	// PointerButtonForward is the browser buttons bit for the browser forward button.
	PointerButtonForward
)

// Pointer identifies a browser pointer event, pointer location, button state and
// modifier state.
//
// X and Y use the same coordinate semantics as [Click.X] and [Click.Y]. Button
// is the changed browser button for down/up events. Buttons is the current
// browser button bitmask and is the preferred field for drag/move handling.
type Pointer struct {
	Name    string
	X       float64
	Y       float64
	Button  int
	Buttons int
	Kind    PointerKind
	Shift   bool
	Control bool
	Alt     bool
}

func (kind PointerKind) String() string {
	switch kind {
	case PointerDown:
		return "down"
	case PointerMove:
		return "move"
	case PointerUp:
		return "up"
	case PointerCancel:
		return "cancel"
	}
	return "PointerKind(" + strconv.Itoa(int(kind)) + ")"
}

func parsePointerKind(value string) (kind PointerKind, ok bool) {
	switch strings.ToLower(value) {
	case "down":
		kind = PointerDown
	case "move":
		kind = PointerMove
	case "up":
		kind = PointerUp
	case "cancel":
		kind = PointerCancel
	}
	ok = kind != 0
	return
}

func (ptr Pointer) keyState() (state int) {
	if ptr.Shift {
		state |= clickKeyShift
	}
	if ptr.Control {
		state |= clickKeyControl
	}
	if ptr.Alt {
		state |= clickKeyAlt
	}
	return
}

func (ptr *Pointer) setKeyState(state int) {
	ptr.Shift = (state & clickKeyShift) != 0
	ptr.Control = (state & clickKeyControl) != 0
	ptr.Alt = (state & clickKeyAlt) != 0
}

// String formats ptr for the JaWS wire protocol.
func (ptr Pointer) String() string {
	return ptr.Kind.String() + " " +
		strconv.FormatFloat(ptr.X, 'g', -1, 64) + " " +
		strconv.FormatFloat(ptr.Y, 'g', -1, 64) + " " +
		strconv.Itoa(ptr.keyState()) + " " +
		strconv.Itoa(ptr.Button) + " " +
		strconv.Itoa(ptr.Buttons) + " " +
		ptr.Name
}

func parsePointerData(value string) (ptr Pointer, after string, ok bool) {
	var pointerPart string
	pointerPart, after, _ = strings.Cut(value, "\t")
	var n int
	var kstate int
	ok = true
	for field := range strings.FieldsSeq(pointerPart) {
		if ok {
			switch n {
			case 0:
				ptr.Kind, ok = parsePointerKind(field)
			case 1:
				ptr.X, ok = runAtof(field)
			case 2:
				ptr.Y, ok = runAtof(field)
			case 3:
				kstate, ok = runAtoi(field)
				ptr.setKeyState(kstate)
			case 4:
				ptr.Button, ok = runAtoi(field)
			case 5:
				ptr.Buttons, ok = runAtoi(field)
			case 6:
				ptr.Name = field
			default:
				ptr.Name += " " + field
			}
			n++
		}
	}
	ok = ok && n >= 6
	return
}
