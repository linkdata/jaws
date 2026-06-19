package ui

import (
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
	Template pageTemplate
}

// pageTemplate wraps a [Template] used as a whole-page document template.
type pageTemplate struct {
	Template
}

// JawsUpdate is a no-op because a page-level template is render-only: the
// embedded [Template.JawsUpdate] would re-render the entire document into itself
// when OuterHTMLTag is set, so it is deliberately silenced here.
func (pageTemplate) JawsUpdate(*jaws.Element) {}

func (h uiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rq := h.NewRequest(r)
	rw := RequestWriter{Request: rq, Writer: w}
	_ = h.Log(rq.NewElement(h.Template).JawsRender(rw, nil))
}

// Handler returns an http.Handler that renders the named template.
//
// The returned handler can be registered directly with a router. Each request
// results in the template being looked up through the configured template
// lookupers and rendered with a [With] value as the template data, exposing
// dot through its Dot field.
func Handler(jw *jaws.Jaws, name string, dot any) http.Handler {
	return uiHandler{Jaws: jw, Template: pageTemplate{Template: Template{Name: name, Dot: dot}}}
}
