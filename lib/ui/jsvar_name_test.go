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
		{name: "name"},
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
