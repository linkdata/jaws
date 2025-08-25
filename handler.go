package jaws

import "net/http"

// Handler implements ServeHTTP with a jaws.Template
type Handler struct {
	*jwsvc
	Template
}

func (h Handler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

// Handler returns a http.Handler using a jaws.Template
func (jw *jwsvc) Handler(name string, dot any) http.Handler {
	return Handler{jwsvc: jw, Template: Template{Name: name, Dot: dot}}
}
