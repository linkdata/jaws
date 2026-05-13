package ui

import (
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
	Template pageTemplate
}

type pageTemplate struct {
	Template
}

func (tmpl pageTemplate) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = tmpl.Template.render(elem, w, params, false)
	return
}

func (pageTemplate) JawsUpdate(*jaws.Element) {}

func (h uiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(w, nil))
}

// Handler returns an http.Handler that renders the named template.
//
// The returned handler can be registered directly with a router. Each request
// results in the template being looked up through the configured template
// lookupers and rendered directly with dot as the template data.
func Handler(jw *jaws.Jaws, name string, dot any) http.Handler {
	return uiHandler{Jaws: jw, Template: pageTemplate{Template: Template{Name: name, Dot: dot}}}
}
