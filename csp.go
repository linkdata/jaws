package jaws

import (
	"net/url"

	"github.com/linkdata/jaws/internal/headresource"
	"github.com/linkdata/secureheaders"
)

func buildContentSecurityPolicy(resourceURLs []*url.URL) (value string) {
	resources := make([]secureheaders.Resource, 0, len(resourceURLs)*2)
	for _, u := range resourceURLs {
		// Automatic inference preserves every conventional permission associated
		// with the configured URL. The explicit destination permits the request
		// emitted in the page head.
		resources = append(resources, secureheaders.Resource{URL: u})

		resource := secureheaders.Resource{URL: u}
		destination, _ := headresource.Classify(u)
		switch destination {
		case headresource.DestinationScript:
			resource.Destination = secureheaders.ResourceDestinationScript
		case headresource.DestinationStyle:
			resource.Destination = secureheaders.ResourceDestinationStyle
		case headresource.DestinationImage:
			resource.Destination = secureheaders.ResourceDestinationImage
		case headresource.DestinationFont:
			resource.Destination = secureheaders.ResourceDestinationFont
		case headresource.DestinationFetch:
			resource.Destination = secureheaders.ResourceDestinationConnect
		default:
			continue
		}
		resources = append(resources, resource)
	}
	value = secureheaders.BuildContentSecurityPolicy(resources...)
	return
}
