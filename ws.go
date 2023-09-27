package jaws

import (
	"bytes"
	"context"
	"net/http"

	"github.com/linkdata/jaws/what"
	"nhooyr.io/websocket"
)

// ServeHTTP implements http.HanderFunc
func (rq *Request) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ws, err := websocket.Accept(w, r, nil); err == nil {
		if err = rq.onConnect(); err == nil {
			incomingMsgCh := make(chan wsMsg)
			broadcastMsgCh := rq.Jaws.subscribe(rq, 1)
			outboundMsgCh := make(chan wsMsg, cap(broadcastMsgCh))
			go wsReader(r.Context(), rq.Jaws.Done(), incomingMsgCh, ws) // closes incomingMsgCh
			go wsWriter(r.Context(), rq.Jaws.Done(), outboundMsgCh, ws) // calls ws.Close()
			rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)    // unsubscribes broadcastMsgCh, closes outboundMsgCh
		} else {
			defer ws.Close(websocket.StatusNormalClosure, err.Error())
			var msg wsMsg
			msg.FillAlert(rq.Jaws.Log(err))
			_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
		}
	} else {
		_ = rq.Jaws.Log(err)
	}
	rq.recycle()
}

// wsReader reads websocket text messages, parses them and sends them on incomingMsgCh.
//
// Closes incomingMsgCh on exit.
func wsReader(ctx context.Context, jawsDoneCh <-chan struct{}, incomingMsgCh chan<- wsMsg, ws *websocket.Conn) {
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
}

// wsParse parses an incoming text buffer into a message.
func wsParse(txt []byte) (wsMsg, bool) {
	txt = bytes.ToValidUTF8(txt, nil) // we don't trust client browsers
	// first newline must not be first charater, that would leave no room for id
	if nl1 := bytes.IndexByte(txt, '\n'); nl1 > 0 {
		if nl2 := bytes.IndexByte(txt[nl1+1:], '\n'); nl2 >= 0 {
			nl2 += nl1 + 1
			return wsMsg{
				Id:   string(txt[0:nl1]),
				What: what.Parse(string(txt[nl1+1 : nl2])),
				Data: string(txt[nl2+1:]),
			}, true
		}
	}
	return wsMsg{}, false
}

// wsWriter reads JaWS messages from outboundMsgCh, formats them and writes them to the websocket.
//
// Closes the websocket on exit.
func wsWriter(ctx context.Context, jawsDoneCh <-chan struct{}, outboundMsgCh <-chan wsMsg, ws *websocket.Conn) {
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
			err = ws.Write(ctx, websocket.MessageText, msg.Append(nil))
		}
	}
}
