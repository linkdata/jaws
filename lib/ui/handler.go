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
	// pageTemplate, so ServeHTTP can construct a fresh request-scoped page UI while
	// accepting arbitrary page data.
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

// statusRecorder wraps an [http.ResponseWriter] to apply the page handler's
// default HTML content type and record whether a final response has been
// committed, so [uiHandler.ServeHTTP] can still send a 500 for a render failure
// that occurred before any output was written.
type statusRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (sr *statusRecorder) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if sr.Header().Get("Content-Type") == "" {
		sr.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	sr.wrote = true
	return sr.ResponseWriter.Write(p)
}

func (sr *statusRecorder) WriteHeader(code int) {
	if code == http.StatusSwitchingProtocols || code >= 200 {
		sr.wrote = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

func (h uiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rq := h.NewRequest(w, r)
	if handler, ok := h.dot.(jaws.ConnectHandler); ok {
		rq.SetConnectFn(handler.JawsConnect)
	}
	sr := &statusRecorder{ResponseWriter: w}
	rw := RequestWriter{Request: rq, Writer: sr}
	// Build a fresh per-request pointer so the UI is comparable as a map key
	// regardless of the page dot: ordinary html/template data such as a slice or map
	// is not usable as a tag and would fail the runtime comparability check in
	// Request.NewElement if a bare pageTemplate value (whose Dot is any) were used.
	// The pointer identity is always comparable and fresh per request. Element tracking
	// lives in the page Element's state slot claimed by pageTemplate.JawsRender.
	// The private constructor bypasses NewTemplate's "div" default. pageTemplate
	// executes the document directly and deliberately emits no generated wrapper.
	pt := &pageTemplate{Template: newTemplate("", h.name, h.dot)}
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
// For each request, Handler looks up name and renders it with [With.Dot] set to
// dot. Unless the response already has a Content-Type, Handler sets it to
// "text/html; charset=utf-8" on the first write. Before rendering, Handler calls
// [jaws.Jaws.NewRequest], which replaces any existing Cache-Control value with
// "no-store". A render failure before any output uses [http.Error]'s text
// response.
//
// Handler renders without a generated wrapper and does not use dot as a tag.
// Dot may be arbitrary template data. When dot implements [jaws.ConnectHandler],
// Handler installs its JawsConnect method on each Request before executing the
// page template. The page GET does not invoke JawsConnect. Only the top-level
// dot's method set is inspected; normal method promotion applies. A
// ConnectHandler on a non-promoted field or a dot passed to a nested [Template]
// is ignored without a diagnostic.
//
// Handler reuses dot across requests. Dot and its callbacks must support
// concurrent execution. The bundled client connects after parsing the document.
// A custom client may dial after flushed response bytes expose the request key,
// so JawsConnect can overlap the initial page render.
func Handler(jw *jaws.Jaws, name string, dot any) http.Handler {
	return uiHandler{Jaws: jw, name: name, dot: dot}
}
