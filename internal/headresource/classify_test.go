package headresource

import (
	"mime"
	"net/url"
	"testing"
)

func TestClassify(t *testing.T) {
	const (
		imageExt    = ".jawsheadimage"
		fontExt     = ".jawsheadfont"
		imageryExt  = ".jawsheadimagery"
		fontLikeExt = ".jawsheadfontlike"
		wasmExt     = ".jawsheadwasm"
	)
	for ext, typ := range map[string]string{
		imageExt:    "IMAGE/jaws-test",
		fontExt:     "FONT/jaws-test",
		imageryExt:  "imagery/jaws-test",
		fontLikeExt: "fontlike/jaws-test",
		wasmExt:     "application/wasm",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			t.Fatalf("AddExtensionType(%q, %q): %v", ext, typ, err)
		}
	}

	for _, tc := range []struct {
		name     string
		u        *url.URL
		want     Destination
		wantMIME string
	}{
		{name: "nil", want: DestinationNone},
		{name: "JavaScript", u: &url.URL{Path: "app.JS"}, want: DestinationScript},
		{name: "stylesheet", u: &url.URL{Path: "app.CSS"}, want: DestinationStyle},
		{name: "uppercase image MIME", u: &url.URL{Path: "image" + imageExt}, want: DestinationImage, wantMIME: "IMAGE/jaws-test"},
		{name: "uppercase font MIME", u: &url.URL{Path: "font" + fontExt}, want: DestinationFont, wantMIME: "FONT/jaws-test"},
		{name: "image-like MIME", u: &url.URL{Path: "image" + imageryExt}, want: DestinationFetch, wantMIME: "imagery/jaws-test"},
		{name: "font-like MIME", u: &url.URL{Path: "font" + fontLikeExt}, want: DestinationFetch, wantMIME: "fontlike/jaws-test"},
		{name: "WebAssembly", u: &url.URL{Path: "module" + wasmExt}, want: DestinationFetch, wantMIME: "application/wasm"},
		{name: "module JavaScript", u: &url.URL{Path: "app.mjs"}, want: DestinationFetch},
		{name: "extensionless", u: &url.URL{Path: "data"}, want: DestinationFetch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, gotMIME := Classify(tc.u)
			if got != tc.want {
				t.Fatalf("Classify(%v) destination = %v, want %v", tc.u, got, tc.want)
			}
			if tc.wantMIME != "" && gotMIME != tc.wantMIME {
				t.Fatalf("Classify(%v) MIME type = %q, want %q", tc.u, gotMIME, tc.wantMIME)
			}
			if tc.u == nil && gotMIME != "" {
				t.Fatalf("Classify(nil) MIME type = %q, want empty", gotMIME)
			}
		})
	}
}
