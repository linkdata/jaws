package tag

import (
	"errors"
	"runtime"
	"testing"
)

func Test_errIllegalTagType_Error(t *testing.T) {
	// The bare sentinel omits type and caller details.
	if got := ErrIllegalTagType.Error(); got != "illegal tag type" {
		t.Fatalf("bare sentinel: got %q, want %q", got, "illegal tag type")
	}
	// A concrete tag reports its type.
	if got := (errIllegalTagType{typeName: "int"}).Error(); got != "illegal tag type int" {
		t.Fatalf("int tag: got %q, want %q", got, "illegal tag type int")
	}
	const withCaller = "illegal tag type string; nearest external caller: example.com/app.render (render.go:42)"
	if got := (errIllegalTagType{typeName: "string", caller: "example.com/app.render (render.go:42)"}).Error(); got != withCaller {
		t.Fatalf("caller hint: got %q, want %q", got, withCaller)
	}
	if !errors.Is(errIllegalTagType{typeName: "int"}, ErrIllegalTagType) {
		t.Fatal("expected errors.Is match against ErrIllegalTagType")
	}
}

func Test_formatExternalCaller(t *testing.T) {
	tests := []struct {
		name     string
		function string
		file     string
		line     int
		want     string
	}{
		{name: "application", function: "example.com/app.render", file: "/private/build/app/render.go", line: 42, want: "example.com/app.render (render.go:42)"},
		{name: "jaws root", function: "github.com/linkdata/jaws.(*Jaws).Dirty", file: "/src/jaws/broadcast.go", line: 130},
		{name: "jaws subpackage", function: "github.com/linkdata/jaws/lib/tag.TagExpand", file: "/src/jaws/lib/tag/tag.go", line: 224},
		{name: "jaws prefix collision", function: "github.com/linkdata/jaws-app.render", file: "/src/app/render.go", line: 9, want: "github.com/linkdata/jaws-app.render (render.go:9)"},
		{name: "reflect", function: "reflect.Value.Call", file: "/go/src/reflect/value.go", line: 365},
		{name: "text template", function: "text/template.(*state).evalCall", file: "/go/src/text/template/exec.go", line: 769},
		{name: "html template", function: "html/template.(*Template).Execute", file: "/go/src/html/template/template.go", line: 121},
		{name: "net http", function: "net/http.HandlerFunc.ServeHTTP", file: "/go/src/net/http/server.go", line: 2322},
		{name: "runtime", function: "runtime.main", file: "/go/src/runtime/proc.go", line: 285},
		{name: "testing", function: "testing.tRunner", file: "/go/src/testing/testing.go", line: 1792},
		{name: "unknown function", file: "/src/app/render.go", line: 9},
		{name: "unknown file", function: "example.com/app.render", line: 9},
		{name: "unknown line", function: "example.com/app.render", file: "/src/app/render.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := runtime.Frame{Function: tt.function, File: tt.file, Line: tt.line}
			if got := formatExternalCaller(frame); got != tt.want {
				t.Errorf("formatExternalCaller() = %q, want %q", got, tt.want)
			}
		})
	}
}
