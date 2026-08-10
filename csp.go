package jaws

import (
	"net/url"
	"slices"
	"strings"

	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/secureheaders"
)

func buildContentSecurityPolicy(resourceURLs []*url.URL) (value string) {
	value = secureheaders.BuildContentSecurityPolicy(resourceURLs)
	var sources []string
	for _, u := range resourceURLs {
		if assets.IsFetchPreload(u) {
			if source := fetchPreloadSource(u); source != "" {
				sources = append(sources, source)
			}
		}
	}
	if len(sources) > 0 {
		value = addConnectSources(value, sources)
	}
	return
}

func fetchPreloadSource(u *url.URL) (source string) {
	if u != nil && u.Host != "" {
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "":
			source = strings.ToLower(u.Host)
		case "http", "https":
			source = scheme + "://" + strings.ToLower(u.Host)
		}
	}
	return
}

func addConnectSources(value string, sources []string) string {
	const prefix = "connect-src "
	directives := strings.Split(value, "; ")
	for i, directive := range directives {
		if fields, ok := strings.CutPrefix(directive, prefix); ok {
			values := strings.Fields(fields)
			values = append(values, sources...)
			slices.Sort(values)
			values = slices.Compact(values)
			directives[i] = prefix + strings.Join(values, " ")
			value = strings.Join(directives, "; ")
			return value
		}
	}
	// A missing connect-src means the dependency's policy shape changed. Keep
	// its complete policy instead of replacing the security baseline here.
	return value
}
