package jawstree

import (
	"net/http"
	"net/url"
	"strconv"
)

const (
	initScriptPath    = "/jaws/.jawstree"
	initScriptPattern = initScriptPath + "/{tree}/{options}"
)

var (
	headerCacheControlNoStore   = []string{"no-store"}
	headerContentTypeJavaScript = []string{"text/javascript"}
)

func isSafeTreeName(tree string) (yes bool) {
	for i := range len(tree) {
		c := tree[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_':
		case c == '$':
		default:
			return false
		}
	}
	return len(tree) > 0
}

func initScriptURL(tree string, options Option) string {
	return initScriptPath + "/" + url.PathEscape(tree) + "/" + strconv.FormatInt(int64(options), 10)
}

func appendInitScript(b []byte, tree string, options Option) []byte {
	// Run the initializer immediately if the document is already parsed (the tree
	// was inserted via a WebSocket update after load) and otherwise defer it to
	// DOMContentLoaded.
	b = append(b, `(function(){var i=function(){window["jawstree_"+`...)
	b = strconv.AppendQuote(b, tree)
	b = append(b, `]=jawstreeNew(`...)
	b = strconv.AppendQuote(b, tree)
	b = append(b, `,window["jawstreeroot_"+`...)
	b = strconv.AppendQuote(b, tree)
	b = append(b, "],"...)
	b = strconv.AppendInt(b, int64(options), 10)
	b = append(b, `);};if(document.readyState==="complete"||document.readyState==="interactive"){i();}else{document.addEventListener("DOMContentLoaded",i);}})();`...)
	return b
}

func serveInitScript(w http.ResponseWriter, r *http.Request) {
	tree := r.PathValue("tree")
	if isSafeTreeName(tree) {
		opt, err := strconv.Atoi(r.PathValue("options"))
		if err == nil && opt >= 0 {
			hdr := w.Header()
			hdr["Cache-Control"] = headerCacheControlNoStore
			hdr["Content-Type"] = headerContentTypeJavaScript
			_, _ = w.Write(appendInitScript(nil, tree, Option(opt))) // #nosec G705
			return
		}
	}
	w.WriteHeader(http.StatusBadRequest)
}
