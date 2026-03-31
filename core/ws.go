package jaws

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/core/wire"
)

func (rq *Request) startServe() (ok bool) {
	return rq.claimed.Load() && rq.running.CompareAndSwap(false, true)
}

func (rq *Request) stopServe() {
	rq.cancel(nil)
	rq.Jaws.recycle(rq)
}

var headerContentTypeJavaScript = []string{"text/javascript"}

// ServeHTTP implements http.HanderFunc.
//
// Requires UseRequest() have been successfully called for the Request.
func (rq *Request) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rq.startServe() {
		defer rq.stopServe()
		if strings.HasSuffix(r.URL.Path, "/noscript") {
			w.WriteHeader(http.StatusNoContent)
			rq.cancel(ErrJavascriptDisabled)
			return
		}
		var err error
		if r.Header.Get("Sec-WebSocket-Key") != "" {
			if err = rq.validateWebSocketOrigin(r); err != nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				rq.cancel(err)
				return
			}
		}
		var ws *websocket.Conn
		ws, err = websocket.Accept(w, r, nil)
		if err == nil {
			if err = rq.onConnect(); err == nil {
				incomingMsgCh := make(chan wire.WsMsg)
				broadcastMsgCh := rq.Jaws.subscribe(rq, 4+len(rq.elems)*4)
				outboundMsgCh := make(chan wire.WsMsg, cap(broadcastMsgCh))
				go wire.ReadLoop(rq.ctx, rq.cancelFn, rq.Jaws.Done(), incomingMsgCh, ws)  // closes incomingMsgCh
				go wire.WriteLoop(rq.ctx, rq.cancelFn, rq.Jaws.Done(), outboundMsgCh, ws) // calls ws.Close()
				rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)                  // unsubscribes broadcastMsgCh, closes outboundMsgCh
			} else {
				defer ws.Close(websocket.StatusNormalClosure, err.Error())
				var msg wire.WsMsg
				msg.FillAlert(rq.Jaws.Log(err))
				_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
			}
		}
		rq.cancel(err)
	}
}
