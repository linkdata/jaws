package jaws_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/linkdata/jaws"
)

func TestJawsSetupIgnoresNilSetupFunc(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	var nilSetup jaws.SetupFunc
	if err = jw.Setup(nil, "", nilSetup); err != nil {
		t.Fatalf("Setup(nil SetupFunc) error = %v, want nil", err)
	}

	firstErr := errors.New("first setup error")
	secondErr := errors.New("second setup error")
	var calls int
	first := jaws.SetupFunc(func(_ *jaws.Jaws, _ jaws.HandleFunc, _ string) (urls []*url.URL, err error) {
		calls++
		err = firstErr
		return
	})
	second := jaws.SetupFunc(func(_ *jaws.Jaws, _ jaws.HandleFunc, _ string) (urls []*url.URL, err error) {
		calls++
		err = secondErr
		return
	})
	err = jw.Setup(nil, "", first, nilSetup, second)
	if calls != 2 {
		t.Fatalf("Setup called %d non-nil setup functions, want 2", calls)
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Setup error = %v, want both setup errors", err)
	}
}
