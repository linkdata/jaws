package jaws

// This file resolves the client IP for a request, honouring trusted forwarded
// headers when [Jaws.TrustForwardedHeaders] is set, and compares addresses
// loopback-aware via equalIP for the tail-fetch and WebSocket binding checks.

import (
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"strings"
)

// equalIP reports whether a and b identify the same client for the purpose of
// session and request-key binding. Addresses are unmapped first so an
// IPv4-mapped IPv6 address (::ffff:a.b.c.d, as a proxy may write into a forwarded
// header) matches its plain IPv4 form. Two loopback addresses always compare equal
// so that a reverse proxy connecting to the backend over loopback does not break
// binding; the consequence is that when every request arrives from loopback (the
// typical proxied deployment without forwarded-IP binding) IP binding is a no-op.
// Enable [Jaws.TrustForwardedHeaders] to bind on the forwarded client IP instead
// (see the clientIP method).
func equalIP(a, b netip.Addr) bool {
	a, b = a.Unmap(), b.Unmap()
	return a.Compare(b) == 0 || (a.IsLoopback() && b.IsLoopback())
}

func parseIP(remoteAddr string) (ip netip.Addr) {
	if remoteAddr != "" {
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			ip, _ = netip.ParseAddr(host)
		} else {
			ip, _ = netip.ParseAddr(remoteAddr)
		}
	}
	return
}

// clientIP returns the address used to bind sessions and request keys to a
// client. When [Jaws.TrustForwardedHeaders] is set it prefers the client IP from
// the proxy-supplied forwarded headers, so binding keeps working behind a reverse
// proxy that connects over loopback; otherwise (and as a fallback) it uses the
// transport peer address. TrustForwardedHeaders must only be enabled behind a
// single reverse proxy you control that sets these headers (see the field doc).
func (jw *Jaws) clientIP(r *http.Request) (ip netip.Addr) {
	if r != nil {
		if jw.TrustForwardedHeaders {
			if fip, ok := forwardedClientIP(r.Header); ok {
				return fip
			}
		}
		ip = parseIP(r.RemoteAddr)
	}
	return
}

// forwardedClientIP extracts the client IP from proxy-supplied headers. It uses
// the leftmost X-Forwarded-For entry (the original client as seen by a single
// trusted proxy), falling back to X-Real-IP. Callers must only trust these
// headers when behind a controlled proxy (see [Jaws.TrustForwardedHeaders]).
func forwardedClientIP(h http.Header) (netip.Addr, bool) {
	if xff := h.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip, err := netip.ParseAddr(textproto.TrimString(first)); err == nil {
			return ip, true
		}
	}
	if xrip := textproto.TrimString(h.Get("X-Real-Ip")); xrip != "" {
		if ip, err := netip.ParseAddr(xrip); err == nil {
			return ip, true
		}
	}
	return netip.Addr{}, false
}
