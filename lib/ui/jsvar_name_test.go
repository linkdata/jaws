package ui

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestValidateJsVarNameRejectsOnlyReservedPrototypeName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "__proto__", wantErr: true},
		{name: "__proto"},
		{name: "__proto__$"},
		{name: "$__proto__"},
		{name: "constructor"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateJsVarName([]any{tc.name})
			if tc.wantErr {
				if !errors.Is(err, ErrIllegalJsVarName) {
					t.Fatalf("validateJsVarName(%q) error = %v, want ErrIllegalJsVarName", tc.name, err)
				}
				return
			}
			if err != nil || got != tc.name {
				t.Fatalf("validateJsVarName(%q) = %q, %v", tc.name, got, err)
			}
		})
	}
}

func TestJsVarRenderRejectsReservedPrototypeName(t *testing.T) {
	_, rq := newCoreRequest(t)
	var mu sync.Mutex
	value := jsVarData{Text: "value"}
	jsvar := NewJsVar(&mu, &value)
	elem := rq.NewElement(jsvar)
	var output strings.Builder

	err := jsvar.JawsRender(elem, &output, []any{"__proto__"})
	if !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("JawsRender error = %v, want ErrIllegalJsVarName", err)
	}
	if output.Len() != 0 {
		t.Fatalf("JawsRender wrote rejected JsVar markup %q", output.String())
	}
}

func TestRequestWriterJsVarRejectsReservedPrototypeName(t *testing.T) {
	_, rq := newCoreRequest(t)
	var mu sync.Mutex
	value := jsVarData{Text: "value"}
	jsvar := NewJsVar(&mu, &value)
	var output strings.Builder
	rw := RequestWriter{Request: rq, Writer: &output}

	err := rw.JsVar("__proto__", jsvar)
	if !errors.Is(err, ErrIllegalJsVarName) {
		t.Fatalf("RequestWriter.JsVar error = %v, want ErrIllegalJsVarName", err)
	}
	if output.Len() != 0 {
		t.Fatalf("RequestWriter.JsVar wrote rejected markup %q", output.String())
	}
}

func TestJsVarRenderAllowsDuplicateNames(t *testing.T) {
	_, rq := newCoreRequest(t)
	var mu sync.Mutex

	// Two bindings sharing a name both render without error; the browser delivers
	// a write to every live binding of the name.
	for _, text := range []string{"first", "second"} {
		v := jsVarData{Text: text}
		jsvar := NewJsVar(&mu, &v)
		elem := rq.NewElement(jsvar)
		var out strings.Builder
		if err := jsvar.JawsRender(elem, &out, []any{"dup"}); err != nil {
			t.Fatalf("JawsRender(%q) error = %v", text, err)
		}
		if out.Len() == 0 {
			t.Fatalf("JawsRender(%q) wrote no markup", text)
		}
	}
}
