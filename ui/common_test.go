package ui

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws/core"
)

func TestCommon_applyDirty(t *testing.T) {
	_, rq := newRequest(t)
	elem, _ := renderUI(t, rq, NewSpan(testHTMLGetter("x")))
	tag := &struct{}{}

	changed, err := applyDirty(tag, elem, nil)
	if err != nil || !changed {
		t.Fatalf("want changed,nil got %v,%v", changed, err)
	}

	changed, err = applyDirty(tag, elem, core.ErrValueUnchanged)
	if err != nil || changed {
		t.Fatalf("want unchanged,nil got %v,%v", changed, err)
	}

	wantErr := errors.New("boom")
	changed, err = applyDirty(tag, elem, wantErr)
	if !errors.Is(err, wantErr) || changed {
		t.Fatalf("want unchanged,%v got %v,%v", wantErr, changed, err)
	}
}
