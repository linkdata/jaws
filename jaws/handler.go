package jaws

import "net/http"

// Handler is an http.Handler that renders a template for every request.
//
// It wires the incoming HTTP request through the JaWS rendering pipeline by
// creating a Request, instantiating the configured Template and streaming the
// resulting HTML to the caller. Applications typically obtain a Handler via the
// Jaws.Handler helper.
type Handler struct {
	*Jaws
	Template
}

func (h Handler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

// Handler returns an http.Handler that renders the named template.
//
// The returned handler can be registered directly with a router. Each request
// results in the template being looked up through the configured Template
// lookupers and rendered with dot as the template data.
func (jw *Jaws) Handler(name string, dot any) http.Handler {
	return Handler{Jaws: jw, Template: Template{Name: name, Dot: dot}}
}
