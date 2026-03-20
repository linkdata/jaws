package core

import (
	"net/url"
	"strings"
	"testing"

	"github.com/linkdata/jaws/secureheaders"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestJaws_GenerateHeadHTML_StoresCSPBuiltBySecureHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.ListenURL = "https://listenurl.com:8443/api/ws"
	extras := []string{
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
		"https://images.example.com/logo.png",
	}
	if err = jw.GenerateHeadHTML(extras...); err != nil {
		t.Fatal(err)
	}

	urls := []*url.URL{
		mustParseURL(t, jw.serveCSS.Name),
		mustParseURL(t, jw.serveJS.Name),
	}
	for _, extra := range extras {
		urls = append(urls, mustParseURL(t, extra))
	}

	wantCSP, err := secureheaders.BuildContentSecurityPolicy(urls, jw.ListenURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := jw.ContentSecurityPolicy(); got != wantCSP {
		t.Fatalf("unexpected CSP:\nwant: %q\ngot:  %q", wantCSP, got)
	}
}

func TestJaws_GenerateHeadHTML_PropagatesCSPErrors(t *testing.T) {
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
