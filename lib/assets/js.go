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

// PreloadHTML returns HTML code to load the given resources efficiently.
//
// JaWS emits .js scripts and .css stylesheets when automatic inference selects
// the matching destination. A trailing @version is ignored when its final
// extension is otherwise unrecognized. Resources inferred only as images or
// fonts by [secureheaders.InferResourceDestinations] are emitted as preloads for
// their respective destinations. Other resources are omitted. Font preloads
// use anonymous CORS. Favicon image URLs are returned separately. A compatible
// MIME type from the final extension is included in the preload's type attribute.
//
// Nil URL arguments are skipped. A resource is returned as faviconURL only when
// its base name begins with "favicon" and it is classified as an image. A script
// or stylesheet is never treated as a favicon, regardless of its base name. If
// more than one resource qualifies as a favicon, the last one wins and earlier
// favicon URLs are discarded rather than emitted as preload links.
func PreloadHTML(urls ...*url.URL) (htmlCode, faviconURL string) {
	var jsurls, cssurls []string
	var favicontype string
	var buf []byte
	for _, u := range urls {
		if u == nil {
			continue
		}
		destinations, recognized := secureheaders.InferResourceDestinations(u)
		if !recognized {
			continue
		}
		urlstr := u.String()
		switch activeResourceExtension(u.Path) {
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
		mimetype, _, _ := strings.Cut(mime.TypeByExtension(path.Ext(u.Path)), ";")
		if !strings.HasPrefix(strings.ToLower(mimetype), asattr+"/") {
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

func activeResourceExtension(pathname string) (ext string) {
	ext = strings.ToLower(path.Ext(pathname))
	if ext == ".js" || ext == ".css" {
		return
	}
	_, name := path.Split(pathname)
	i := strings.LastIndexByte(name, '@')
	if i <= 0 || i >= len(name)-1 {
		ext = ""
		return
	}
	versionedExt := strings.ToLower(path.Ext(name[:i]))
	if versionedExt != ".js" && versionedExt != ".css" {
		ext = ""
		return
	}
	// Preserve secureheaders' ordinary-final-extension precedence before
	// interpreting a trailing @suffix as a version.
	if !strings.ContainsRune(ext, '@') {
		if _, recognized := secureheaders.InferResourceDestinations(&url.URL{Path: "resource" + ext}); recognized {
			ext = ""
			return
		}
	}
	ext = versionedExt
	return
}
