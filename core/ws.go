package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

func (rq *Request) startServe() (ok bool) {
	return rq.claimed.Load() && rq.running.CompareAndSwap(false, true)
}

func (rq *Request) stopServe() {
	rq.cancel(nil)
	rq.Jaws.recycle(rq)
}

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
				incomingMsgCh := make(chan WsMsg)
				broadcastMsgCh := rq.Jaws.subscribe(rq, 4+len(rq.elems)*4)
				outboundMsgCh := make(chan WsMsg, cap(broadcastMsgCh))
				go wsReader(rq.ctx, rq.cancelFn, rq.Jaws.Done(), incomingMsgCh, ws) // closes incomingMsgCh
				go wsWriter(rq.ctx, rq.cancelFn, rq.Jaws.Done(), outboundMsgCh, ws) // calls ws.Close()
				rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)            // unsubscribes broadcastMsgCh, closes outboundMsgCh
			} else {
				defer ws.Close(websocket.StatusNormalClosure, err.Error())
				var msg WsMsg
				msg.FillAlert(rq.Jaws.Log(err))
				_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
			}
		}
		rq.cancel(err)
	}
}

// wsReader reads websocket text messages, parses them and sends them on incomingMsgCh.
//
// Closes incomingMsgCh on exit.
func wsReader(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, incomingMsgCh chan<- WsMsg, ws *websocket.Conn) {
	var typ websocket.MessageType
	var txt []byte
	var err error
	defer close(incomingMsgCh)
	for err == nil {
		if typ, txt, err = ws.Read(ctx); typ == websocket.MessageText {
			if msg, ok := wsParse(txt); ok {
				select {
				case <-ctx.Done():
					return
				case <-jawsDoneCh:
					return
				case incomingMsgCh <- msg:
				}
			}
		}
	}
	if ccf != nil {
		ccf(err)
	}
}

// wsWriter reads JaWS messages from outboundMsgCh, formats them and writes them to the websocket.
//
// Closes the websocket on exit.
func wsWriter(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, outboundMsgCh <-chan WsMsg, ws *websocket.Conn) {
	defer ws.Close(websocket.StatusNormalClosure, "")
	var err error
	for err == nil {
		select {
		case <-ctx.Done():
			return
		case <-jawsDoneCh:
			return
		case msg, ok := <-outboundMsgCh:
			if !ok {
				return
			}
			var wc io.WriteCloser
			if wc, err = ws.Writer(ctx, websocket.MessageText); err == nil {
				err = wsWriteData(wc, msg, outboundMsgCh)
			}
		}
	}
	if ccf != nil {
		ccf(err)
	}
}

func wsWriteData(wc io.WriteCloser, firstMsg WsMsg, outboundMsgCh <-chan WsMsg) (err error) {
	b := firstMsg.Append(nil)
	// accumulate data to send as long as more messages
	// are available until it exceeds 32K
batchloop:
	for len(b) < 32*1024 {
		select {
		case msg, ok := <-outboundMsgCh:
			if !ok {
				break batchloop
			}
			b = msg.Append(b)
		default:
			break batchloop
		}
	}
	_, err = wc.Write(b)
	err = errors.Join(err, wc.Close())
	return
}
