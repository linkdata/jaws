package tag

import (
	"errors"
	"testing"
)

func Test_errIllegalTagType_Error(t *testing.T) {
	// The bare sentinel has a nil tag and omits the "<nil>" type.
	if got := ErrIllegalTagType.Error(); got != "illegal tag type" {
		t.Fatalf("nil tag: got %q, want %q", got, "illegal tag type")
	}
	// A concrete tag reports its type.
	if got := (errIllegalTagType{tag: 5}).Error(); got != "illegal tag type int" {
		t.Fatalf("int tag: got %q, want %q", got, "illegal tag type int")
	}
	if !errors.Is(errIllegalTagType{tag: 5}, ErrIllegalTagType) {
		t.Fatal("expected errors.Is match against ErrIllegalTagType")
	}
}
