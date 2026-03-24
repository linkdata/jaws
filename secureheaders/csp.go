package secureheaders

import (
	"mime"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// BuildContentSecurityPolicy returns a CSP header value based on resource URLs.
//
// Resource URLs contribute external source expressions to script, style, image,
// font and connect directives according to their type.
func BuildContentSecurityPolicy(resourceURLs []*url.URL) (value string, err error) {
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
					// Stylesheets commonly reference webfonts via relative URLs.
					fontSrc[source] = struct{}{}
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

	value = strings.Join([]string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		cspDirective("script-src", []string{"'self'"}, scriptSrc),
		cspDirective("style-src", []string{"'self'", "'unsafe-inline'"}, styleSrc),
		cspDirective("img-src", []string{"'self'", "data:"}, imgSrc),
		cspDirective("font-src", []string{"'self'"}, fontSrc),
		cspDirective("connect-src", []string{"'self'"}, connectSrc),
	}, "; ")

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

	mimetype := mime.TypeByExtension(filepath.Ext(u.Path))
	switch {
	case strings.HasPrefix(mimetype, "text/css"):
		return "style"
	case strings.HasPrefix(mimetype, "text/javascript"):
		return "script"
	case strings.HasPrefix(mimetype, "image/"):
		return "img"
	case strings.HasPrefix(mimetype, "font/"):
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
