package jaws

import (
	"mime"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/linkdata/secureheaders"
)

func TestBuildContentSecurityPolicy_KnownResourcesUnchanged(t *testing.T) {
	const (
		imageExt = ".jawscspimage"
		fontExt  = ".jawscspfont"
	)
	for ext, typ := range map[string]string{
		imageExt: "image/jaws-test",
		fontExt:  "font/jaws-test",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			t.Fatalf("AddExtensionType(%q, %q): %v", ext, typ, err)
		}
	}

	urls := []*url.URL{
		mustParseURL(t, "https://scripts.example.test/app.js"),
		mustParseURL(t, "https://styles.example.test/app.css"),
		mustParseURL(t, "https://images.example.test/favicon"+imageExt),
		mustParseURL(t, "https://fonts.example.test/font"+fontExt),
		mustParseURL(t, "wss://events.example.test/socket"),
	}
	want := secureheaders.BuildContentSecurityPolicy(urls)
	if got := buildContentSecurityPolicy(urls); got != want {
		t.Fatalf("buildContentSecurityPolicy() = %q, want %q", got, want)
	}
}

func TestBuildContentSecurityPolicy_FetchResources(t *testing.T) {
	const ext = ".jawscspresource"
	if err := mime.AddExtensionType(ext, "application/wasm"); err != nil {
		t.Fatal(err)
	}

	urls := []*url.URL{
		mustParseURL(t, "https://CDN.Example.test:8443/module"+ext),
		mustParseURL(t, "https://cdn.example.test:8443/worker"+ext),
		mustParseURL(t, "https://cdn.example.test:8443/module.mjs"),
		mustParseURL(t, "https://data.example.test/resource"),
		mustParseURL(t, "//Protocol.Example.test:9443/resource"),
		mustParseURL(t, "/local"+ext),
		mustParseURL(t, "wss://events.example.test/socket"),
	}
	csp := buildContentSecurityPolicy(urls)
	directives := strings.Split(csp, "; ")
	var connectValues []string
	for _, directive := range directives {
		if values, ok := strings.CutPrefix(directive, "connect-src "); ok {
			connectValues = strings.Fields(values)
		}
	}
	wantConnectValues := []string{
		"'self'",
		"https://cdn.example.test:8443",
		"https://data.example.test",
		"protocol.example.test:9443",
		"wss://events.example.test",
	}
	if !slices.Equal(connectValues, wantConnectValues) {
		t.Fatalf("connect-src values = %q, want %q; CSP: %s", connectValues, wantConnectValues, csp)
	}
	if got := strings.Count(csp, "https://data.example.test"); got != 1 {
		t.Fatalf("generic fetch origin occurs %d times, want 1; CSP: %s", got, csp)
	}
	if !strings.Contains(csp, "script-src 'self' https://cdn.example.test:8443") {
		t.Fatalf("CSP does not retain the module script source: %s", csp)
	}
}

func TestFetchPreloadSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "HTTPS", raw: "https://CDN.Example.test:8443/path", want: "https://cdn.example.test:8443"},
		{name: "HTTP", raw: "HTTP://Example.test/path", want: "http://example.test"},
		{name: "scheme relative", raw: "//CDN.Example.test:8443/path", want: "cdn.example.test:8443"},
		{name: "WebSocket", raw: "wss://events.example.test/socket"},
		{name: "relative", raw: "/relative/path"},
		{name: "nil"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var u *url.URL
			if tc.raw != "" {
				u = mustParseURL(t, tc.raw)
			}
			if got := fetchPreloadSource(u); got != tc.want {
				t.Fatalf("fetchPreloadSource(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestAddConnectSources(t *testing.T) {
	t.Run("sorts and deduplicates", func(t *testing.T) {
		const policy = "default-src 'self'; connect-src 'self' wss://events.example.test"
		sources := []string{
			"https://z.example.test",
			"https://a.example.test",
			"https://a.example.test",
			"wss://events.example.test",
		}
		const want = "default-src 'self'; connect-src 'self' https://a.example.test https://z.example.test wss://events.example.test"
		if got := addConnectSources(policy, sources); got != want {
			t.Fatalf("addConnectSources() = %q, want %q", got, want)
		}
	})

	t.Run("missing directive", func(t *testing.T) {
		const policy = "default-src 'self'; script-src 'self'"
		if got := addConnectSources(policy, []string{"https://cdn.example.test"}); got != policy {
			t.Fatalf("addConnectSources() = %q, want unchanged %q", got, policy)
		}
	})
}
