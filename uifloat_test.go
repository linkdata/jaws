package jaws

import (
	"sync"
	"testing"
)

func TestUiFloat(t *testing.T) {
	var l sync.Mutex
	var rl sync.RWMutex
	var val float64

	ui := UiFloat{L: &l, P: &val}

	if ui.JawsGetFloat(nil) != 0 {
		t.Fail()
	}

	if x := ui.JawsSetFloat(nil, -1); x != nil {
		t.Error(x)
	}

	if x := ui.JawsSetFloat(nil, ui.JawsGetFloat(nil)); x != ErrValueUnchanged {
		t.Error(x)
	}

	ui.L = &rl

	if ui.JawsGetFloat(nil) != -1 {
		t.Fail()
	}
}
