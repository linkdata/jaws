package core

import "net/http"

type sessioner struct {
	jw *Jaws
	h  http.Handler
}

func (sess sessioner) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	if sess.jw.GetSession(r) == nil {
		sess.jw.newSession(wr, r)
	}
	sess.h.ServeHTTP(wr, r)
}

// Session returns a http.Handler that ensures a JaWS Session exists before invoking h.
func (jw *Jaws) Session(h http.Handler) http.Handler {
	return sessioner{jw: jw, h: h}
}
