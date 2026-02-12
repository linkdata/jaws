package ui

import (
	"net/http"

	"github.com/linkdata/jaws/core"
)

// uiHandler is an http.uiHandler that renders a template for every request.
//
// It wires the incoming HTTP request through the JaWS rendering pipeline by
// creating a Request, instantiating the configured Template and streaming the
// resulting HTML to the caller. Applications typically construct handlers with
// Handler.
type uiHandler struct {
	*core.Jaws
	Template
}

func (h uiHandler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

// Handler returns an http.Handler that renders the named template.
//
// The returned handler can be registered directly with a router. Each request
// results in the template being looked up through the configured Template
// lookupers and rendered with dot as the template data.
func Handler(jw *core.Jaws, name string, dot any) http.Handler {
	return uiHandler{Jaws: jw, Template: Template{Name: name, Dot: dot}}
}
