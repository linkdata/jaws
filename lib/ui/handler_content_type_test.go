package ui

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
)

func TestHandler_DefaultsContentTypeToHTML(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"bom", "\ufeff<!doctype html><html><body>hello</body></html>"},
		{"element", "<main>hello</main>"},
		{"doctype", "<!doctype html><html><body>hello</body></html>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			if err = jw.AddTemplateLookuper(template.Must(template.New("page").Parse(tc.body))); err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			Handler(jw, "page", nil).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

			if got, want := rr.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got := rr.Body.String(); got != tc.body {
				t.Fatalf("body = %q, want %q", got, tc.body)
			}
		})
	}
}

func TestHandler_PreservesExplicitContentType(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	if err = jw.AddTemplateLookuper(template.Must(template.New("page").Parse("<main>hello</main>"))); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	rr.Header().Set("Content-Type", "application/xhtml+xml")
	Handler(jw, "page", nil).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := rr.Header().Get("Content-Type"), "application/xhtml+xml"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
}

func TestHandler_RenderFailureBeforeOutputUsesTextContentType(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Logger = new(templateLogger)
	t.Cleanup(jw.Close)

	rr := httptest.NewRecorder()
	Handler(jw, "missing", nil).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := rr.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rr.Header().Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
}
