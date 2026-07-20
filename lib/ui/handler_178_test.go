package ui

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// TestHandler_StringDotRenders is the regression test for issue #178: a plain
// string page dot must render as ordinary html/template data rather than being
// rejected as an illegal JaWS tag, which previously produced a blank HTTP 200.
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

// TestHandler_StructDotRenders verifies that a non-tag struct-pointer page dot
// (illegal as a tag) renders normally through the page handler.
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

// TestHandler_MissingTemplateReturns500 verifies that a render failure occurring
// before any output (an unresolved template) produces an observable HTTP 500 and
// logs the error, rather than a blank HTTP 200.
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

// TestHandler_ExecuteErrorAfterOutputKeepsPartialBody verifies the best-effort
// streaming contract: once bytes have been committed, a later execution error
// leaves the partial body and its already-sent 200 status in place.
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

// TestHandler_PartialTemplateStillRejectsStringDot documents the intended
// contrast with the page handler: a partial ui.Template continues to derive tag
// identity from its dot, so a plain string dot is still rejected as an illegal
// tag.
func TestHandler_PartialTemplateStillRejectsStringDot(t *testing.T) {
	jw, rq := newCoreRequest(t)
	_ = jw.AddTemplateLookuper(template.Must(template.New("partial").Parse(`{{.Dot}}`)))

	var sb bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &sb}
	if err := rw.Template("div", "partial", "plain-string-dot"); !errors.Is(err, tag.ErrIllegalTagType) {
		t.Fatalf("partial template err = %v, want %v", err, tag.ErrIllegalTagType)
	}
}

// TestHandler_TagValuedDotStillRenders guards that a valid tag value as the page
// dot keeps rendering (its Dot passthrough is unchanged; only the now-inert tag
// registration is skipped).
func TestHandler_TagValuedDotStillRenders(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	_ = jw.AddTemplateLookuper(template.Must(template.New("page").Parse(
		`<html><body>{{with $.Dot}}<span>{{.}}</span>{{end}}</body></html>`,
	)))

	rr := httptest.NewRecorder()
	Handler(jw, "page", tag.Tag("ok")).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); !strings.Contains(got, `<span>ok</span>`) {
		t.Fatalf("body = %q, want it to contain <span>ok</span>", got)
	}
}
