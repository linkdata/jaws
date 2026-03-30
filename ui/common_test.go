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

	err := applyDirty(tag, elem, nil)
	if err != nil {
		t.Fatalf("want nil got %v", err)
	}

	err = applyDirty(tag, elem, core.ErrValueUnchanged)
	if err != nil {
		t.Fatalf("want nil got %v", err)
	}

	wantErr := errors.New("boom")
	err = applyDirty(tag, elem, wantErr)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v got %v", wantErr, err)
	}
}
