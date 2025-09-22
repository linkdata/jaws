package jaws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		if strings.HasSuffix(r.RequestURI, "/noscript") {
			w.WriteHeader(http.StatusNoContent)
			rq.cancel(nil)
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
				incomingMsgCh := make(chan wsMsg)
				broadcastMsgCh := rq.Jaws.subscribe(rq, 4+len(rq.elems)*4)
				outboundMsgCh := make(chan wsMsg, cap(broadcastMsgCh))
				go wsReader(rq.ctx, rq.cancelFn, rq.Jaws.Done(), incomingMsgCh, ws) // closes incomingMsgCh
				go wsWriter(rq.ctx, rq.cancelFn, rq.Jaws.Done(), outboundMsgCh, ws) // calls ws.Close()
				rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)            // unsubscribes broadcastMsgCh, closes outboundMsgCh
			} else {
				defer ws.Close(websocket.StatusNormalClosure, err.Error())
				var msg wsMsg
				msg.FillAlert(rq.Jaws.Log(err))
				_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
			}
		}
		rq.cancel(err)
	}
}

func requestHost(req *http.Request) string {
	if req == nil {
		return ""
	}
	if host := req.Host; host != "" {
		return host
	}
	if req.URL != nil && req.URL.Host != "" {
		return req.URL.Host
	}
	return ""
}

func (rq *Request) validateWebSocketOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return errors.New("websocket request missing Origin header")
	}
	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("invalid websocket Origin %q: %w", origin, err)
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "http", "https":
	default:
		return fmt.Errorf("websocket Origin %q must use http or https", origin)
	}
	if u.Host == "" {
		return fmt.Errorf("websocket Origin %q missing host", origin)
	}
	expectedHost := requestHost(r)
	if initial := rq.Initial(); initial != nil {
		if host := requestHost(initial); host != "" {
			expectedHost = host
		}
	}
	if expectedHost == "" {
		return errors.New("unable to determine expected websocket origin host")
	}
	if !strings.EqualFold(u.Host, expectedHost) {
		return fmt.Errorf("websocket Origin host %q is not allowed", u.Host)
	}
	return nil
}

// wsReader reads websocket text messages, parses them and sends them on incomingMsgCh.
//
// Closes incomingMsgCh on exit.
func wsReader(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, incomingMsgCh chan<- wsMsg, ws *websocket.Conn) {
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
func wsWriter(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, outboundMsgCh <-chan wsMsg, ws *websocket.Conn) {
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

func wsWriteData(wc io.WriteCloser, firstMsg wsMsg, outboundMsgCh <-chan wsMsg) (err error) {
	b := firstMsg.Append(nil)
	// accumulate data to send as long as more messages
	// are available until it exceeds 32K
batchloop:
	for len(b) < 32*1024 {
		select {
		case msg := <-outboundMsgCh:
			b = msg.Append(b)
		default:
			break batchloop
		}
	}
	_, err = wc.Write(b)
	err = errors.Join(err, wc.Close())
	return
}
