package ui

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// TestHandler_StringDotRenders renders a plain string page dot as ordinary
// html/template data (issue #178).
func TestHandler_StringDotRenders(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	_ = jw.AddTemplateLookuper(template.Must(template.New("page").Parse(`hello {{.Dot}}`)))

	rr := httptest.NewRecorder()
	Handler(jw, "page", "world").ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "hello world" {
		t.Fatalf("body = %q, want %q", got, "hello world")
	}
}

type pageDot struct {
	Field string
}

// TestHandler_StructDotRenders renders a struct-pointer page dot.
func TestHandler_StructDotRenders(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	_ = jw.AddTemplateLookuper(template.Must(template.New("page").Parse(`value={{.Dot.Field}}`)))

	rr := httptest.NewRecorder()
	Handler(jw, "page", &pageDot{Field: "x"}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "value=x" {
		t.Fatalf("body = %q, want %q", got, "value=x")
	}
}

// TestHandler_NonComparableDotRenders renders slice and map page dots. Such
// values are legal html/template data but are not usable as tags and are not
// comparable as map keys, so the per-request pointer wrapper is required to keep
// the UI comparable for Request.NewElement's -race comparability check.
func TestHandler_NonComparableDotRenders(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	_ = jw.AddTemplateLookuper(template.Must(template.New("page").Parse(`{{range .Dot}}{{.}}{{end}}`)))

	for _, tc := range []struct {
		name string
		dot  any
		want string
	}{
		{"slice", []string{"a", "b", "c"}, "abc"},
		{"map", map[string]string{"only": "x"}, "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Guard that the dot genuinely exercises the non-comparable path.
			if tag.NewErrNotComparable(tc.dot) == nil {
				t.Fatalf("dot %T is comparable; test does not exercise the fix", tc.dot)
			}
			rr := httptest.NewRecorder()
			Handler(jw, "page", tc.dot).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			if got := rr.Body.String(); got != tc.want {
				t.Fatalf("body = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestHandler_MissingTemplateReturns500 checks that a render failure before any
// output (an unresolved template) returns HTTP 500 and logs the error.
func TestHandler_MissingTemplateReturns500(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := new(templateLogger)
	jw.Logger = logger

	rr := httptest.NewRecorder()
	Handler(jw, "nope", "x").ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected a non-empty error body")
	}
	if len(logger.errors) != 1 || !errors.Is(logger.errors[0], ErrMissingTemplate) {
		t.Fatalf("logged errors = %#v, want one %v", logger.errors, ErrMissingTemplate)
	}
}

// TestHandler_ExecuteErrorAfterOutputKeepsPartialBody checks the best-effort
// execution contract: once output has been written, a later execution error
// leaves the partial body and its committed 200 status in place.
func TestHandler_ExecuteErrorAfterOutputKeepsPartialBody(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := new(templateLogger)
	jw.Logger = logger

	renderErr := errors.New("boom")
	_ = jw.AddTemplateLookuper(template.Must(template.New("page").Funcs(template.FuncMap{
		"fail": func() (string, error) { return "", renderErr },
	}).Parse(`prefix{{fail}}`)))

	rr := httptest.NewRecorder()
	Handler(jw, "page", "x").ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (partial body already committed)", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "prefix" {
		t.Fatalf("body = %q, want %q", got, "prefix")
	}
	if len(logger.errors) != 1 || !errors.Is(logger.errors[0], renderErr) {
		t.Fatalf("logged errors = %#v, want one %v", logger.errors, renderErr)
	}
}

// TestHandler_PartialTemplateStillRejectsStringDot documents that a partial
// ui.Template derives tag identity from its dot, so a plain string dot is
// rejected as an illegal tag while the whole-page Handler accepts one.
func TestHandler_PartialTemplateStillRejectsStringDot(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("partial").Parse(`{{.Dot}}`)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("div", "partial", "plain-string-dot"); !errors.Is(err, tag.ErrIllegalTagType) {
		t.Fatalf("partial template err = %v, want %v", err, tag.ErrIllegalTagType)
	}
}
