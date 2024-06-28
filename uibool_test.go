package jaws

import (
	"sync"
	"testing"
)

func TestUiBool(t *testing.T) {
	var l sync.Mutex
	var rl sync.RWMutex
	var val bool

	ui := UiBool{L: &l, P: &val}

	if ui.JawsGetBool(nil) {
		t.Fail()
	}

	if x := ui.JawsSetBool(nil, true); x != nil {
		t.Error(x)
	}

	if x := ui.JawsSetBool(nil, ui.JawsGetBool(nil)); x != ErrValueUnchanged {
		t.Error(x)
	}

	ui.L = &rl

	if !ui.JawsGetBool(nil) {
		t.Fail()
	}
}
