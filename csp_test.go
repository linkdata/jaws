package jaws

import (
	"mime"
	"net/url"
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
	}
	resources := make([]secureheaders.Resource, 0, len(urls))
	for _, u := range urls {
		resources = append(resources, secureheaders.Resource{URL: u})
	}
	want := secureheaders.BuildContentSecurityPolicy(resources...)
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
	resources := make([]secureheaders.Resource, 0, len(urls)*2)
	for _, u := range urls {
		resources = append(resources, secureheaders.Resource{URL: u})
		resources = append(resources, secureheaders.Resource{
			URL:         u,
			Destination: secureheaders.ResourceDestinationConnect,
		})
	}
	want := secureheaders.BuildContentSecurityPolicy(resources...)
	if got := buildContentSecurityPolicy(urls); got != want {
		t.Fatalf("buildContentSecurityPolicy() = %q, want %q", got, want)
	}
}
