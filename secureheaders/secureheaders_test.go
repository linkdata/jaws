package secureheaders_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws/secureheaders"
)

var wantDefaultHeaders = map[string]string{
	"Referrer-Policy":         "strict-origin-when-cross-origin",
	"Content-Security-Policy": "default-src 'self'; frame-ancestors 'none'",
	"X-Content-Type-Options":  "nosniff",
	"X-Frame-Options":         "DENY",
	"X-Xss-Protection":        "0",
	"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
}

func assertDefaultHeaders(t *testing.T, hdr http.Header, wantHSTS bool) {
	t.Helper()

	for key, want := range wantDefaultHeaders {
		if got := hdr.Get(key); got != want {
			t.Errorf("%s: expected %q, got %q", key, want, got)
		}
	}

	if wantHSTS {
		if got := hdr.Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
			t.Errorf("Strict-Transport-Security: expected %q, got %q", "max-age=31536000; includeSubDomains", got)
		}
	} else if got := hdr.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security: expected empty, got %q", got)
	}
}

func TestDefaultWriteHeaders(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		rr := httptest.NewRecorder()
		secureheaders.DefaultSetHeaders(rr, false)
		assertDefaultHeaders(t, rr.Header(), false)
	})

	t.Run("https", func(t *testing.T) {
		rr := httptest.NewRecorder()
		secureheaders.DefaultSetHeaders(rr, true)
		assertDefaultHeaders(t, rr.Header(), true)
	})
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		request  func() *http.Request
		wantHSTS bool
	}{
		{
			name: "http",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
			},
			wantHSTS: false,
		},
		{
			name: "https",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
			},
			wantHSTS: true,
		},
		{
			name: "x-forwarded-ssl",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Ssl", "on")
				return r
			},
			wantHSTS: true,
		},
		{
			name: "front-end-https",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Front-End-Https", "on")
				return r
			},
			wantHSTS: true,
		},
		{
			name: "x-forwarded-proto-list",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Proto", "http, https")
				return r
			},
			wantHSTS: false,
		},
		{
			name: "forwarded-proto",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Forwarded", "for=192.0.2.1;proto=https;by=203.0.113.9")
				return r
			},
			wantHSTS: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := secureheaders.Middleware{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}), TrustForwardedHeaders: true}

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, tc.request())

			if !called {
				t.Fatal("expected wrapped handler to be called")
			}
			if sc := rr.Result().StatusCode; sc != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, sc)
			}
			assertDefaultHeaders(t, rr.Result().Header, tc.wantHSTS)
		})
	}
}

func TestMiddlewareWithTrustForwardedHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")

	h := secureheaders.Middleware{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
	})}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	assertDefaultHeaders(t, rr.Result().Header, false)
}

func TestRequestIsSecure(t *testing.T) {
	tests := []struct {
		name                  string
		request               func() *http.Request
		trustForwardedHeaders bool
		want                  bool
	}{
		{
			name: "nil",
			request: func() *http.Request {
				return nil
			},
			trustForwardedHeaders: false,
			want:                  false,
		},
		{
			name: "tls",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
			},
			trustForwardedHeaders: false,
			want:                  true,
		},
		{
			name: "forwarded-disabled",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Proto", "https")
				return r
			},
			trustForwardedHeaders: false,
			want:                  false,
		},
		{
			name: "x-forwarded-ssl",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Ssl", "on")
				return r
			},
			trustForwardedHeaders: true,
			want:                  true,
		},
		{
			name: "front-end-https",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Front-End-Https", "on")
				return r
			},
			trustForwardedHeaders: true,
			want:                  true,
		},
		{
			name: "x-forwarded-proto-first-hop-http",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Proto", "http, https")
				return r
			},
			trustForwardedHeaders: true,
			want:                  false,
		},
		{
			name: "x-forwarded-proto-first-hop-https",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Proto", "https, http")
				return r
			},
			trustForwardedHeaders: true,
			want:                  true,
		},
		{
			name: "x-forwarded-proto-first-hop-https-with-extra-whitespace-token",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("X-Forwarded-Proto", "https nonsense")
				return r
			},
			trustForwardedHeaders: true,
			want:                  true,
		},
		{
			name: "forwarded-proto-https",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Forwarded", "for=192.0.2.1;proto=https;by=203.0.113.9")
				return r
			},
			trustForwardedHeaders: true,
			want:                  true,
		},
		{
			name: "forwarded-proto-http",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Forwarded", "for=192.0.2.1;proto=http")
				return r
			},
			trustForwardedHeaders: true,
			want:                  false,
		},
		{
			name: "forwarded-first-hop-wins",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
				r.Header.Set("Forwarded", "for=192.0.2.1;proto=http, for=198.51.100.2;proto=https")
				return r
			},
			trustForwardedHeaders: true,
			want:                  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := secureheaders.RequestIsSecure(tc.request(), tc.trustForwardedHeaders)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
