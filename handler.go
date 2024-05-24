package jaws

import "net/http"

// Handler implements ServeHTTP with a jaws.Template
type Handler struct {
	*Jaws
	Template
}

func (h Handler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

// Handler returns a http.Handler using a jaws.Template
func (jw *Jaws) Handler(name string, dot any) http.Handler {
	return Handler{Jaws: jw, Template: Template{Template: name, Dot: dot}}
}
