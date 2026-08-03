package ui

import (
	"html/template"
	"io"
	"net/http"

	"github.com/linkdata/jaws"
)

// uiHandler is an http.Handler that renders a template for every request.
//
// It wires the incoming HTTP request through the JaWS rendering pipeline by
// creating a Request, instantiating the configured page template and streaming
// the resulting HTML to the caller. Applications typically construct handlers
// with Handler.
type uiHandler struct {
	*jaws.Jaws
	// name and dot are the page template's parameters rather than a prepared
	// pageTemplate: a pageTemplate tracks the Elements of one render, so ServeHTTP
	// constructs a fresh one per request instead of copying a shared value.
	name string
	dot  any
}

// pageTemplate wraps a [Template] used as a whole-page document template.
type pageTemplate struct {
	Template
}

// The per-request page UI is a *pageTemplate; see [uiHandler.ServeHTTP].
var _ jaws.UI = (*pageTemplate)(nil)

// JawsUpdate is a no-op because a page-level template is render-only: the
// embedded [Template.JawsUpdate] would re-render the entire document into itself
// when OuterHTMLTag is set, so it is deliberately silenced here.
func (*pageTemplate) JawsUpdate(*jaws.Element) {}

// JawsRender renders the whole-page template, looking it up and executing it
// directly.
//
// Unlike the embedded [Template], the page dot is ordinary [html/template] data
// and is never treated as a JaWS tag: there is no tag expansion, no generated
// wrapper element, and [pageTemplate.JawsUpdate] is a no-op. Because the page
// element cannot re-render itself, deriving tag identity from the page dot would
// serve no purpose; nested UI created during execution registers its own tags
// independently.
func (pt *pageTemplate) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	// Claim the state slot here rather than relying on Template.render, which this
	// bypasses: without a claim the page's nested UI would be tracked by nothing at all.
	// This is the only renderer of the page Element, so the claim always succeeds.
	st := &templateState{}
	if err = jaws.SetElementState(elem, st); err == nil {
		var lookedUp *template.Template
		if lookedUp, err = pt.lookup(elem); err == nil {
			// A failed execution needs no cleanup here: RequestWriter.NewUI unregisters
			// the page Element and, through the state slot, everything it owns.
			err = pt.execute(elem, w, lookedUp, st)
		}
	}
	return
}

// statusRecorder wraps an [http.ResponseWriter] to record whether any response
// bytes have been committed, so [uiHandler.ServeHTTP] can still send a 500 for a
// render failure that occurred before any output was written.
type statusRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (sr *statusRecorder) Write(p []byte) (int, error) {
	sr.wrote = true
	return sr.ResponseWriter.Write(p)
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.wrote = true
	sr.ResponseWriter.WriteHeader(code)
}

func (h uiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rq := h.NewRequest(r)
	sr := &statusRecorder{ResponseWriter: w}
	rw := RequestWriter{Request: rq, Writer: sr}
	// Build a fresh per-request pointer so the UI is comparable as a map key
	// regardless of the page dot: ordinary html/template data such as a slice or map
	// is not usable as a tag and would fail the runtime comparability check in
	// Request.NewElement if a bare pageTemplate value (whose Dot is any) were used.
	// The pointer identity is always comparable and fresh per request, and each
	// request gets its own Element-tracking state.
	pt := &pageTemplate{Template: Template{Name: h.name, Dot: h.dot}}
	if err := rw.NewUI(pt); err != nil {
		_ = h.Log(err)
		// A failure before any output (for example a missing template) can still
		// become a proper error response; once bytes have been written the status
		// is already committed and the partial body is left as-is, matching the
		// best-effort execution semantics documented on Template.
		if !sr.wrote {
			http.Error(sr, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

// Handler returns an http.Handler that renders the named template.
//
// The returned handler can be registered directly with a router. Each request
// results in the template being looked up through the configured template
// lookupers and rendered with a [With] value as the template data, exposing
// dot through its Dot field.
func Handler(jw *jaws.Jaws, name string, dot any) http.Handler {
	return uiHandler{Jaws: jw, name: name, dot: dot}
}
