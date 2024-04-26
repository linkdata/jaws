package jaws

import (
	"sync"
	"testing"
)

func TestUiBool_Locker(t *testing.T) {
	var val bool
	var mu sync.Mutex

	ui := UiBool{L: &mu, P: &val}

	if ui.JawsGetBool(nil) {
		t.Fail()
	}

	if x := ui.JawsSetBool(nil, true); x != nil {
		t.Error(x)
	}

	if !ui.JawsGetBool(nil) {
		t.Fail()
	}
}

func TestUiBool_RLocker(t *testing.T) {
	var val bool
	var mu sync.RWMutex

	ui := UiBool{L: &mu, P: &val}

	if ui.JawsGetBool(nil) {
		t.Fail()
	}

	if x := ui.JawsSetBool(nil, true); x != nil {
		t.Error(x)
	}

	if !ui.JawsGetBool(nil) {
		t.Fail()
	}
}
