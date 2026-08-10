// Package headresource classifies resources emitted in generated page heads.
package headresource

import (
	"mime"
	"net/url"
	"path"
	"strings"
)

// Destination identifies how generated head markup requests a resource.
type Destination uint8

const (
	// DestinationNone identifies no resource.
	DestinationNone Destination = iota

	// DestinationScript identifies a script resource.
	DestinationScript

	// DestinationStyle identifies a stylesheet resource.
	DestinationStyle

	// DestinationImage identifies an image resource.
	DestinationImage

	// DestinationFont identifies a font resource.
	DestinationFont

	// DestinationFetch identifies a generic fetch resource.
	DestinationFetch
)

// Classify returns the head request destination and MIME type for u.
//
// A nil URL returns [DestinationNone] and an empty MIME type.
func Classify(u *url.URL) (destination Destination, mimetype string) {
	if u != nil {
		ext := strings.ToLower(path.Ext(u.Path))
		mimetype = mime.TypeByExtension(ext)
		mimetype, _, _ = strings.Cut(mimetype, ";")
		switch ext {
		case ".js":
			destination = DestinationScript
		case ".css":
			destination = DestinationStyle
		default:
			// Match MIME families case-insensitively at the type/subtype boundary.
			switch lowmime := strings.ToLower(mimetype); {
			case strings.HasPrefix(lowmime, "image/"):
				destination = DestinationImage
			case strings.HasPrefix(lowmime, "font/"):
				destination = DestinationFont
			default:
				// Fetch is the generic destination when the URL and MIME type
				// provide no more specific one.
				destination = DestinationFetch
			}
		}
	}
	return
}
