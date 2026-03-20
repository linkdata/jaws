package core

import (
	"strings"
	"testing"
)

func TestJaws_ContentSecurityPolicy_Default(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	want := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	if got := jw.ContentSecurityPolicy(); got != want {
		t.Fatalf("unexpected default CSP header:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestJaws_GenerateHeadHTML_StoresCSPWithExternalSourcesAndListenURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.ListenURL = "https://listenurl.com:8443/api/ws"
	err = jw.GenerateHeadHTML(
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
		"https://cdn.jsdelivr.net/npm/bootstrap-icons/font/fonts/bootstrap-icons.woff2",
		"https://images.example.com/logo.png",
	)
	if err != nil {
		t.Fatal(err)
	}

	csp := jw.ContentSecurityPolicy()
	if !strings.Contains(csp, "script-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected script-src to include cdn source, got: %q", csp)
	}
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net") {
		t.Fatalf("expected style-src to include cdn source, got: %q", csp)
	}
	if !strings.Contains(csp, "font-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected font-src to include cdn source, got: %q", csp)
	}
	if !strings.Contains(csp, "img-src 'self' data: https://images.example.com") {
		t.Fatalf("expected img-src to include image host, got: %q", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' wss://listenurl.com:8443") {
		t.Fatalf("expected connect-src to include wss source from ListenURL, got: %q", csp)
	}
}

func TestJaws_GenerateHeadHTML_InvalidListenURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.ListenURL = "https://bad host"
	err = jw.GenerateHeadHTML()
	if err == nil {
		t.Fatal("expected parse error for ListenURL")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}
