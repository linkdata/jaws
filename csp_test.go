package jaws

import (
	"mime"
	"net/url"
	"testing"
)

func TestBuildContentSecurityPolicy_HeadDestinations(t *testing.T) {
	const (
		imageExt = ".jawscspimage"
		fontExt  = ".jawscspfont"
		wasmExt  = ".jawscspwasm"
	)
	for ext, typ := range map[string]string{
		imageExt: "IMAGE/jaws-test",
		fontExt:  "FONT/jaws-test",
		wasmExt:  "application/wasm",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			t.Fatalf("AddExtensionType(%q, %q): %v", ext, typ, err)
		}
	}

	urls := []*url.URL{
		nil,
		mustParseURL(t, "https://scripts.example.test/app.js"),
		mustParseURL(t, "https://styles.example.test/app.css"),
		mustParseURL(t, "https://images.example.test/image"+imageExt),
		mustParseURL(t, "https://fonts.example.test/font"+fontExt),
		mustParseURL(t, "https://api.example.test/module"+wasmExt),
		mustParseURL(t, "https://modules.example.test/module.mjs"),
	}
	const want = "default-src 'self'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"script-src 'self' https://modules.example.test https://scripts.example.test; " +
		"style-src 'self' 'unsafe-inline' https://styles.example.test; " +
		"img-src 'self' data: https://images.example.test; " +
		"font-src 'self' https://fonts.example.test https://styles.example.test; " +
		"connect-src 'self' https://api.example.test https://modules.example.test"
	if got := buildContentSecurityPolicy(urls); got != want {
		t.Fatalf("buildContentSecurityPolicy() = %q, want %q", got, want)
	}
}
