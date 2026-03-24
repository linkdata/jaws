package secureheaders_test

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

func TestSecureHeaders_BuildContentSecurityPolicy_Default(t *testing.T) {
	got, err := secureheaders.BuildContentSecurityPolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "default-src 'self'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'"
	if got != want {
		t.Fatalf("unexpected default CSP:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestSecureHeaders_BuildContentSecurityPolicy_ExternalResources(t *testing.T) {
	urls := []*url.URL{
		mustParseURL(t, "https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css"),
		mustParseURL(t, "https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js"),
		mustParseURL(t, "https://cdn.jsdelivr.net/npm/bootstrap-icons/font/fonts/bootstrap-icons.woff2"),
		mustParseURL(t, "https://images.example.com/logo.png"),
	}
	got, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "script-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected script-src to include cdn source, got: %q", got)
	}
	if !strings.Contains(got, "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net") {
		t.Fatalf("expected style-src to include cdn source, got: %q", got)
	}
	if !strings.Contains(got, "font-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected font-src to include cdn source, got: %q", got)
	}
	if !strings.Contains(got, "img-src 'self' data: https://images.example.com") {
		t.Fatalf("expected img-src to include image source, got: %q", got)
	}
	if !strings.Contains(got, "connect-src 'self'") {
		t.Fatalf("expected connect-src self baseline, got: %q", got)
	}
}

func TestSecureHeaders_BuildContentSecurityPolicy_ConnectResource(t *testing.T) {
	urls := []*url.URL{
		mustParseURL(t, "wss://events.example.com/socket"),
		mustParseURL(t, "https://cdn.example.com/asset.unknownext"),
	}
	got, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "connect-src 'self' wss://events.example.com") {
		t.Fatalf("expected connect-src to include wss source, got: %q", got)
	}
	if strings.Contains(got, "cdn.example.com") {
		t.Fatalf("unexpected unsupported resource source in CSP: %q", got)
	}
}

func TestSecureHeaders_BuildContentSecurityPolicy_StyleSourceAlsoAllowsFonts(t *testing.T) {
	urls := []*url.URL{
		mustParseURL(t, "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.5/font/bootstrap-icons.min.css"),
	}
	got, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "font-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected stylesheet source to be added to font-src, got: %q", got)
	}
}

func TestSecureHeaders_BuildContentSecurityPolicy_FontExtensionWithQuery(t *testing.T) {
	urls := []*url.URL{
		mustParseURL(t, "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.5/font/fonts/bootstrap-icons.woff2?1fa40e"),
	}
	got, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "font-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected explicit .woff2 source in font-src, got: %q", got)
	}
}

func TestSecureHeaders_BuildContentSecurityPolicy_FontByMIMEExtension(t *testing.T) {
	urls := []*url.URL{
		mustParseURL(t, "https://cdn.jsdelivr.net/fonts/family.ttc"),
	}
	got, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "font-src 'self' https://cdn.jsdelivr.net") {
		t.Fatalf("expected .ttc source in font-src via MIME detection, got: %q", got)
	}
}
