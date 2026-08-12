package jaws

import (
	"errors"
	"reflect"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

func TestJaws_MustTagExpand(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	if got := jw.MustTagExpand([]any{tag.Tag("a"), tag.Tag("b")}); !reflect.DeepEqual(got, []any{tag.Tag("a"), tag.Tag("b")}) {
		t.Fatalf("MustTagExpand = %#v, want both tags", got)
	}
	awaitTestLoggerQueue(t, jw)
	if logged := logger.snapshot(); len(logged) != 0 {
		t.Fatalf("successful expansion logged %v, want nothing", logged)
	}
}

// TestJaws_MustTagExpandLogsAndReturnsPartial locks in the documented behavior with a
// Logger configured: the error is reported and the tags expanded before the failure are
// still returned, so the caller applies a partial result rather than nothing.
func TestJaws_MustTagExpandLogsAndReturnsPartial(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	// "bad" is a plain string, an illegal tag type, so expansion fails after
	// tag.Tag("ok") has already been collected.
	got := jw.MustTagExpand([]any{tag.Tag("ok"), "bad"})
	loggedErr := logger.next(t)
	if !errors.Is(loggedErr, tag.ErrIllegalTagType) {
		t.Fatalf("logged error = %v, want %v", loggedErr, tag.ErrIllegalTagType)
	}
	if !reflect.DeepEqual(got, []any{tag.Tag("ok")}) {
		t.Fatalf("MustTagExpand = %#v, want the partial result [tag.Tag(\"ok\")]", got)
	}
}

// TestJaws_MustTagExpandPanicsWithoutLogger is the other half of the contract: with no
// Logger, Jaws.MustLog panics, so the partial result never reaches the caller.
func TestJaws_MustTagExpandPanicsWithoutLogger(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	if jw.Logger != nil {
		t.Fatal("expected nil Logger by default")
	}

	defer func() {
		x := recover()
		if e, ok := x.(error); !ok || !errors.Is(e, tag.ErrIllegalTagType) {
			t.Fatalf("recovered %#v, want an error matching %v", x, tag.ErrIllegalTagType)
		}
	}()
	jw.MustTagExpand([]any{tag.Tag("ok"), "bad"})
	t.Fatal("MustTagExpand returned instead of panicking")
}
