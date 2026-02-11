package ui

import (
	"errors"
	"testing"

	pkg "github.com/linkdata/jaws/jaws"
)

func TestCommon_applyDirty(t *testing.T) {
	_, rq := newRequest(t)
	elem, _ := renderUI(t, rq, NewSpan(testHTMLGetter("x")))
	tag := &struct{}{}

	changed, err := applyDirty(tag, elem, nil)
	if err != nil || !changed {
		t.Fatalf("want changed,nil got %v,%v", changed, err)
	}

	changed, err = applyDirty(tag, elem, pkg.ErrValueUnchanged)
	if err != nil || changed {
		t.Fatalf("want unchanged,nil got %v,%v", changed, err)
	}

	wantErr := errors.New("boom")
	changed, err = applyDirty(tag, elem, wantErr)
	if !errors.Is(err, wantErr) || changed {
		t.Fatalf("want unchanged,%v got %v,%v", wantErr, changed, err)
	}
}

func TestCommon_must(t *testing.T) {
	must(nil)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	must(errors.New("panic"))
}
