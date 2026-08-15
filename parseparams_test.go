package jaws

import (
	"math"
	"testing"
)

type parseParamsFloatClickHandler struct {
	value float64
}

func (parseParamsFloatClickHandler) JawsClick(*Element, Click) error {
	return nil
}

func TestParseParamsNonReflexiveHandlerNotTagged(t *testing.T) {
	h := parseParamsFloatClickHandler{value: math.NaN()}
	tags, handlers, attrs := ParseParams([]any{h})
	if len(tags) != 0 {
		t.Fatalf("tags = %#v, want none", tags)
	}
	if len(handlers) != 1 {
		t.Fatalf("handlers = %#v, want one", handlers)
	}
	if got, ok := handlers[0].(parseParamsFloatClickHandler); !ok || !math.IsNaN(got.value) {
		t.Fatalf("handler = %#v, want original NaN-bearing handler", handlers[0])
	}
	if len(attrs) != 0 {
		t.Fatalf("attrs = %#v, want none", attrs)
	}
}
