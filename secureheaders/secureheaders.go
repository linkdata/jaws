package secureheaders

import (
	"net/http"
	"strings"
)

var headerReferrerPolicy = []string{"strict-origin-when-cross-origin"}
var headerContentSecurityPolicy = []string{"default-src 'self'; frame-ancestors 'none'"}
var headerXContentTypeOptions = []string{"nosniff"}
var headerXFrameOptions = []string{"DENY"}
var headerXXssProtection = []string{"0"}
var headerPermissionsPolicy = []string{"camera=(), microphone=(), geolocation=(), payment=()"}
var headerStrictTransportSecurity = []string{"max-age=31536000; includeSubDomains"}

// SetHeaders is called by the middleware to set response headers.
// It defaults to DefaultSetHeaders.
var SetHeaders = DefaultSetHeaders

// DefaultSetHeaders sets a secure baseline of HTTP response headers.
//
// If ishttps is true, it also sets Strict-Transport-Security.
func DefaultSetHeaders(hw http.ResponseWriter, ishttps bool) {
	hdr := hw.Header()
	hdr["Referrer-Policy"] = headerReferrerPolicy
	hdr["Content-Security-Policy"] = headerContentSecurityPolicy
	hdr["X-Content-Type-Options"] = headerXContentTypeOptions
	hdr["X-Frame-Options"] = headerXFrameOptions
	hdr["X-Xss-Protection"] = headerXXssProtection
	hdr["Permissions-Policy"] = headerPermissionsPolicy
	if ishttps {
		hdr["Strict-Transport-Security"] = headerStrictTransportSecurity
	}
}

// Middleware wraps an HTTP handler and sets secure default response headers
// before delegating to the wrapped handler.
//
// The embedded Handler must be non-nil.
type Middleware struct {
	http.Handler // Handler receives the request after security headers are set.
	// TrustForwardedHeaders enables forwarded-header HTTPS detection
	// (X-Forwarded-Ssl, Front-End-Https, X-Forwarded-Proto and Forwarded).
	// Enable only when these headers are set and sanitized by trusted
	// infrastructure.
	TrustForwardedHeaders bool
}

func (m Middleware) ServeHTTP(hw http.ResponseWriter, hr *http.Request) {
	SetHeaders(hw, RequestIsSecure(hr, m.TrustForwardedHeaders))
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
