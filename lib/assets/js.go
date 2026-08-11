package assets

import (
	_ "embed" // enable go:embed directives
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/secureheaders"
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

// PreloadHTML returns head markup for common resource URLs.
//
// A .js extension matched by [secureheaders.InferResource] emits a deferred
// classic script, and a matched .css extension emits a stylesheet. These forms
// may include a trailing @version when the final extension is otherwise
// unrecognized. Non-favicon resources inferred exclusively as images or fonts
// emit preloads. Module scripts, MIME-only scripts and stylesheets, generic
// fetch resources, and all other URLs are omitted. Font preloads use anonymous
// CORS. Image and font preloads include a compatible MIME type from the final
// extension when available. Resource URLs must come from trusted application
// configuration because matched scripts are executable.
//
// Nil URL arguments are skipped. A resource is returned as faviconURL only when
// its base name begins with "favicon" and it is classified as an image. A script
// or stylesheet is never treated as a favicon, regardless of its base name. If
// more than one resource qualifies as a favicon, the last one wins and earlier
// favicon URLs are discarded. The winner is emitted as an icon and returned as
// faviconURL.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		if u == nil {
			continue
		}
		destinations, matchedExtension := secureheaders.InferResource(u)
		urlstr := u.String()
		switch matchedExtension {
		case ".js":
			if destinations&secureheaders.ResourceDestinationScript != 0 {
				jsurls = append(jsurls, urlstr)
				continue
			}
		case ".css":
			if destinations&secureheaders.ResourceDestinationStyle != 0 {
				cssurls = append(cssurls, urlstr)
				continue
			}
		}
		var asattr string
		switch destinations {
		case secureheaders.ResourceDestinationImage:
			asattr = "image"
		case secureheaders.ResourceDestinationFont:
			asattr = "font"
		default:
			continue
		}
		mimetype, _, mimeErr := mime.ParseMediaType(mime.TypeByExtension(path.Ext(u.Path)))
		if mimeErr != nil || !strings.HasPrefix(mimetype, asattr+"/") {
			mimetype = ""
		}
		if destinations == secureheaders.ResourceDestinationImage && strings.HasPrefix(strings.ToLower(path.Base(u.Path)), "favicon") {
			favicontype = mimetype
			faviconURL = urlstr
			continue
		}
		buf = append(buf, `<link rel="preload"`...)
		buf = htmlio.AppendAttr(buf, "href", urlstr)
		buf = htmlio.AppendAttr(buf, "as", asattr)
		if mimetype != "" {
			buf = htmlio.AppendAttr(buf, "type", mimetype)
		}
		if destinations == secureheaders.ResourceDestinationFont {
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
		if favicontype != "" {
			buf = htmlio.AppendAttr(buf, "type", favicontype)
		}
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
