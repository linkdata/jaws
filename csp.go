package jaws

import (
	"net/url"

	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/secureheaders"
)

func buildContentSecurityPolicy(resourceURLs []*url.URL) (value string) {
	resources := make([]secureheaders.Resource, 0, len(resourceURLs)*2)
	for _, u := range resourceURLs {
		// Automatic inference may permit a URL in another directive; the fetch
		// preload itself also requires connect-src.
		resources = append(resources, secureheaders.Resource{URL: u})
		if assets.IsFetchPreload(u) {
			resources = append(resources, secureheaders.Resource{
				URL:         u,
				Destination: secureheaders.ResourceDestinationConnect,
			})
		}
	}
	value = secureheaders.BuildContentSecurityPolicy(resources...)
	return
}
