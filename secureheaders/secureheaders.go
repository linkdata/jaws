package secureheaders

import (
	"net/http"
	"strings"
)

// DefaultHeaders contains the default security header values used by SetHeaders.
//
// These are not protected by a mutex, so modifying the map while serving requests is racy.
var DefaultHeaders = http.Header{
	"Referrer-Policy":           {"strict-origin-when-cross-origin"},
	"Content-Security-Policy":   {"default-src 'self'; frame-ancestors 'none'"},
	"X-Content-Type-Options":    {"nosniff"},
	"X-Frame-Options":           {"DENY"},
	"X-Xss-Protection":          {"0"},
	"Permissions-Policy":        {"camera=(), microphone=(), geolocation=(), payment=()"},
	"Strict-Transport-Security": {"max-age=31536000; includeSubDomains"},
}

// SetHeaders sets the response headers to the values in src.
// If src is nil, DefaultHeaders is used.
//
// If ishttps is false, Strict-Transport-Security is not set.
func SetHeaders(src http.Header, hw http.ResponseWriter, ishttps bool) {
	if src == nil {
		src = DefaultHeaders
	}
	hdr := hw.Header()
	for k, v := range src {
		if ishttps || k != "Strict-Transport-Security" {
			hdr[k] = v
		}
	}
}

// Middleware wraps an HTTP handler and sets secure default response headers
// before delegating to the wrapped handler.
//
// The embedded Handler must be non-nil.
type Middleware struct {
	http.Handler // Handler receives the request after security headers are set.
	http.Header  // The headers to set. If nil, uses DefaultHeaders
	// TrustForwardedHeaders enables forwarded-header HTTPS detection
	// (X-Forwarded-Ssl, Front-End-Https, X-Forwarded-Proto and Forwarded).
	// Enable only when these headers are set and sanitized by trusted
	// infrastructure.
	TrustForwardedHeaders bool
}

func (m Middleware) ServeHTTP(hw http.ResponseWriter, hr *http.Request) {
	SetHeaders(m.Header, hw, RequestIsSecure(hr, m.TrustForwardedHeaders))
	m.Handler.ServeHTTP(hw, hr)
}

// RequestIsSecure reports if a request should be considered HTTPS.
//
// It always treats requests with non-nil TLS as secure.
//
// If trustForwardedHeaders is true, it also honors the forwarding headers
// X-Forwarded-Ssl, Front-End-Https, X-Forwarded-Proto and Forwarded.
//
// For list-valued forwarding headers, only the first hop is used.
func RequestIsSecure(hr *http.Request, trustForwardedHeaders bool) (yes bool) {
	if hr != nil {
		if hr.TLS != nil {
			yes = true
		}
		if trustForwardedHeaders && !yes {
			yes = headerToken(hr.Header.Get("X-Forwarded-Ssl")) == "on"
			yes = yes || headerToken(hr.Header.Get("Front-End-Https")) == "on"
			yes = yes || requestForwardedIsSecure(hr.Header.Get("Forwarded"))
			yes = yes || requestXForwardedProtoIsSecure(hr.Header.Get("X-Forwarded-Proto"))
		}
	}
	return
}

func requestXForwardedProtoIsSecure(value string) (yes bool) {
	if value != "" {
		if i := strings.IndexByte(value, ','); i >= 0 {
			value = value[:i]
		}
		value = headerToken(value)
		if i := strings.IndexAny(value, " \t\r\n"); i >= 0 {
			value = value[:i]
		}
		yes = value == "https"
	}
	return
}

func requestForwardedIsSecure(value string) (yes bool) {
	if value != "" {
		if i := strings.IndexByte(value, ','); i >= 0 {
			value = value[:i]
		}
		for _, param := range strings.Split(value, ";") {
			param = strings.TrimSpace(param)
			key, val, ok := strings.Cut(param, "=")
			if ok && headerToken(key) == "proto" {
				val = strings.Trim(strings.TrimSpace(val), "\"")
				yes = headerToken(val) == "https"
				return
			}
		}
	}
	return
}

func headerToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
