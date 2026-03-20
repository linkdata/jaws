package secureheaders

import (
	"mime"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// BuildContentSecurityPolicy returns a CSP header value based on resources and listener URL.
//
// Resource URLs contribute external source expressions to script, style, image,
// font and connect directives according to their type.
//
// If listenURL is non-empty and parseable with a host, an additional websocket source
// expression is added to connect-src using that host.
func BuildContentSecurityPolicy(resourceURLs []*url.URL, listenURL string) (value string, err error) {
	scriptSrc := make(map[string]struct{})
	styleSrc := make(map[string]struct{})
	imgSrc := make(map[string]struct{})
	fontSrc := make(map[string]struct{})
	connectSrc := make(map[string]struct{})

	for _, u := range resourceURLs {
		if u != nil {
			if source := cspSourceExpr(u); source != "" {
				switch cspDirectiveForURL(u) {
				case "script":
					scriptSrc[source] = struct{}{}
				case "style":
					styleSrc[source] = struct{}{}
				case "img":
					imgSrc[source] = struct{}{}
				case "font":
					fontSrc[source] = struct{}{}
				case "connect":
					connectSrc[source] = struct{}{}
				}
			}
		}
	}

	var listenSource string
	if listenSource, err = cspListenWebSocketSource(listenURL); err == nil {
		if listenSource != "" {
			connectSrc[listenSource] = struct{}{}
		}
		value = strings.Join([]string{
			"default-src 'self'",
			cspDirective("script-src", []string{"'self'"}, scriptSrc),
			cspDirective("style-src", []string{"'self'", "'unsafe-inline'"}, styleSrc),
			cspDirective("img-src", []string{"'self'", "data:"}, imgSrc),
			cspDirective("font-src", []string{"'self'"}, fontSrc),
			cspDirective("connect-src", []string{"'self'"}, connectSrc),
			"frame-ancestors 'none'",
			"object-src 'none'",
			"base-uri 'self'",
			"form-action 'self'",
		}, "; ")
	}

	return
}

func cspDirective(name string, defaults []string, extras map[string]struct{}) string {
	var values []string
	for extra := range extras {
		values = append(values, extra)
	}
	slices.Sort(values)
	return name + " " + strings.Join(append(defaults, values...), " ")
}

func cspDirectiveForURL(u *url.URL) string {
	switch u.Scheme {
	case "ws", "wss":
		return "connect"
	}

	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".js":
		return "script"
	case ".css":
		return "style"
	}

	mimetype := mime.TypeByExtension(ext)
	if strings.HasPrefix(mimetype, "image/") {
		return "img"
	}
	if strings.HasPrefix(mimetype, "font/") {
		return "font"
	}
	return ""
}

func cspSourceExpr(u *url.URL) (src string) {
	if u.Host != "" {
		switch u.Scheme {
		case "http", "https", "ws", "wss":
			src = u.Scheme + "://" + u.Host
		}
	}
	return
}

func cspListenWebSocketSource(listenURL string) (src string, err error) {
	var u *url.URL
	if u, err = url.Parse(listenURL); err == nil && u.Host != "" {
		switch u.Scheme {
		case "https":
			src = "wss://"
		case "http":
			src = "ws://"
		}
		src += u.Host
	}
	return
}
