package jaws

import (
	"sync"
	"testing"
)

func TestUiString(t *testing.T) {
	var l sync.Mutex
	var rl sync.RWMutex
	var val string

	ui := UiString{L: &l, P: &val}

	if ui.JawsGetString(nil) != "" {
		t.Fail()
	}

	if x := ui.JawsSetString(nil, "foo<"); x != nil {
		t.Error(x)
	}

	if x := ui.JawsSetString(nil, ui.JawsGetString(nil)); x != ErrValueUnchanged {
		t.Error(x)
	}

	ui.L = &rl

	if ui.JawsGetHtml(nil) != "foo&lt;" {
		t.Fail()
	}
}
