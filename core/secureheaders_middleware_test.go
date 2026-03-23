package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws/secureheaders"
)

func TestJaws_SecureHeadersMiddleware_UsesJawsCSP(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err = jw.GenerateHeadHTML(
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
	); err != nil {
		t.Fatal(err)
	}
	wantCSP := jw.ContentSecurityPolicy()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	rr := httptest.NewRecorder()
	jw.SecureHeadersMiddleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatal("expected wrapped handler to be called")
	}
	if got := rr.Result().StatusCode; got != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, got)
	}

	hdr := rr.Result().Header
	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
	if got := hdr.Get("Strict-Transport-Security"); got != secureheaders.DefaultHeaders.Get("Strict-Transport-Security") {
		t.Fatalf("expected HSTS %q, got %q", secureheaders.DefaultHeaders.Get("Strict-Transport-Security"), got)
	}
}

func TestJaws_SecureHeadersMiddleware_ClonesDefaultHeaders(t *testing.T) {
	orig := secureheaders.DefaultHeaders.Clone()
	defer func() {
		secureheaders.DefaultHeaders = orig
	}()

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	wantCSP := jw.ContentSecurityPolicy()
	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	secureheaders.DefaultHeaders.Set("X-Frame-Options", "SAMEORIGIN")
	secureheaders.DefaultHeaders.Set("Content-Security-Policy", "default-src 'none'")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	hdr := rr.Result().Header

	if got := hdr.Get("X-Frame-Options"); got != orig.Get("X-Frame-Options") {
		t.Fatalf("expected X-Frame-Options %q, got %q", orig.Get("X-Frame-Options"), got)
	}
	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
}

func TestJaws_SecureHeadersMiddleware_DoesNotTrustForwardedHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if got := rr.Result().Header.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS over HTTP request with forwarded proto, got %q", got)
	}
}
