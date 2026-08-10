package assets

import (
	_ "embed" // enable go:embed directives
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/linkdata/jaws/lib/htmlio"
)

// JavascriptText is the source code for the client-side JaWS JavaScript library.
//
// It holds the embedded asset as an immutable string shared process-wide.
//
//go:embed jaws.js
var JavascriptText string

// JawsCSS is the source code for the client-side JaWS stylesheet.
//
// It holds the embedded asset as an immutable string shared process-wide.
//
//go:embed jaws.css
var JawsCSS string

type resourceDestination uint8

const (
	resourceDestinationNone resourceDestination = iota
	resourceDestinationScript
	resourceDestinationStyle
	resourceDestinationImage
	resourceDestinationFont
	resourceDestinationFetch
)

func resourceMetadata(u *url.URL) (destination resourceDestination, mimetype string) {
	if u != nil {
		ext := strings.ToLower(path.Ext(u.Path))
		mimetype = mime.TypeByExtension(ext)
		mimetype, _, _ = strings.Cut(mimetype, ";")
		switch ext {
		case ".js":
			destination = resourceDestinationScript
		case ".css":
			destination = resourceDestinationStyle
		default:
			switch lowmime := strings.ToLower(mimetype); {
			case strings.HasPrefix(lowmime, "image/"):
				destination = resourceDestinationImage
			case strings.HasPrefix(lowmime, "font/"):
				destination = resourceDestinationFont
			default:
				destination = resourceDestinationFetch
			}
		}
	}
	return
}

// IsFetchPreload reports whether [PreloadHTML] emits u with the fetch destination.
//
// A nil URL returns false.
func IsFetchPreload(u *url.URL) (yes bool) {
	destination, _ := resourceMetadata(u)
	yes = destination == resourceDestinationFetch
	return
}

// PreloadHTML returns HTML code to load the given resources efficiently.
//
// JavaScript and CSS files are emitted as script and stylesheet tags. Image and
// font resources are emitted as preloads for their respective destinations;
// other resources use the fetch destination. Font and fetch preloads use
// anonymous CORS. Favicon image URLs are returned separately. A recognized MIME
// type is included in the preload's type attribute.
//
// Nil URL arguments are skipped. A resource is returned as faviconURL only when
// its base name begins with "favicon" and its MIME type (resolved from the file
// extension) is image/*. A .js or .css resource is always emitted as a script or
// stylesheet and is never treated as a favicon, regardless of its base name. If
// more than one resource qualifies as a favicon, the last one wins and earlier
// favicon URLs are discarded rather than emitted as preload links.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		destination, mimetype := resourceMetadata(u)
		if destination == resourceDestinationNone {
			continue
		}
		var asattr string
		var crossorigin bool
		urlstr := u.String()
		switch destination {
		case resourceDestinationScript:
			jsurls = append(jsurls, urlstr)
			continue
		case resourceDestinationStyle:
			cssurls = append(cssurls, urlstr)
			continue
		case resourceDestinationFetch:
			asattr = "fetch"
			crossorigin = true
		case resourceDestinationImage:
			asattr = "image"
			if strings.HasPrefix(strings.ToLower(path.Base(u.Path)), "favicon") {
				favicontype = mimetype
				faviconURL = urlstr
				continue
			}
		case resourceDestinationFont:
			asattr = "font"
			crossorigin = true
		}
		buf = append(buf, `<link rel="preload"`...)
		buf = htmlio.AppendAttr(buf, "href", urlstr)
		if asattr != "" {
			buf = htmlio.AppendAttr(buf, "as", asattr)
		}
		if mimetype != "" {
			buf = htmlio.AppendAttr(buf, "type", mimetype)
		}
		if crossorigin {
			buf = htmlio.AppendAttr(buf, "crossorigin", "anonymous")
		}
		buf = append(buf, ">\n"...)
	}
	for _, urlstr := range cssurls {
		buf = append(buf, `<link rel="stylesheet"`...)
		buf = htmlio.AppendAttr(buf, "href", urlstr)
		buf = append(buf, ">\n"...)
	}
	if faviconURL != "" {
		buf = append(buf, `<link rel="icon"`...)
		buf = htmlio.AppendAttr(buf, "type", favicontype)
		buf = htmlio.AppendAttr(buf, "href", faviconURL)
		buf = append(buf, ">\n"...)
	}
	for _, urlstr := range jsurls {
		buf = append(buf, `<script defer`...)
		buf = htmlio.AppendAttr(buf, "src", urlstr)
		buf = append(buf, "></script>\n"...)
	}
	htmlCode = string(buf)
	return
}
