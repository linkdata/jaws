package ui

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestHandler_EmptyWriteBeforeErrorReturns500(t *testing.T) {
	renderErr := errors.New("boom")
	tmpl := template.Must(template.New("page").Funcs(template.FuncMap{
		"fail": func() (string, error) { return "", renderErr },
	}).Parse(`{{.Dot}}{{fail}}`))

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			if err = jw.AddTemplateLookuper(tmpl); err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			Handler(jw, "page", "").ServeHTTP(rr, httptest.NewRequest(method, "/", nil))

			if got, want := rr.Code, http.StatusInternalServerError; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
			if got, want := rr.Header().Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
		})
	}
}

type recordingResponseWriter struct {
	header      http.Header
	headerCodes []int
	writes      int
}

func (rw *recordingResponseWriter) Header() http.Header {
	return rw.header
}

func (rw *recordingResponseWriter) Write(p []byte) (int, error) {
	rw.writes++
	return len(p), nil
}

func (rw *recordingResponseWriter) WriteHeader(code int) {
	rw.headerCodes = append(rw.headerCodes, code)
}

func TestStatusRecorder_EmptyWriteDoesNotCommit(t *testing.T) {
	rw := &recordingResponseWriter{header: make(http.Header)}
	sr := statusRecorder{ResponseWriter: rw}

	n, err := sr.Write(nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("Write returned %d bytes, want 0", n)
	}
	if sr.wrote {
		t.Fatal("empty Write marked the response committed")
	}
	if rw.writes != 0 {
		t.Fatalf("underlying Write calls = %d, want 0", rw.writes)
	}
	if got := rw.header.Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}

func TestStatusRecorder_InformationalHeaderCommitState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  int
		wrote bool
	}{
		{"continue", http.StatusContinue, false},
		{"processing", http.StatusProcessing, false},
		{"early hints", http.StatusEarlyHints, false},
		{"last informational", 199, false},
		{"switching protocols", http.StatusSwitchingProtocols, true},
		{"success", http.StatusOK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rw := &recordingResponseWriter{header: make(http.Header)}
			sr := statusRecorder{ResponseWriter: rw}

			sr.WriteHeader(tc.code)

			if sr.wrote != tc.wrote {
				t.Fatalf("wrote = %t, want %t", sr.wrote, tc.wrote)
			}
			if want := []int{tc.code}; !slices.Equal(rw.headerCodes, want) {
				t.Fatalf("underlying WriteHeader calls = %v, want %v", rw.headerCodes, want)
			}
		})
	}
}
