package wire

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/coder/websocket"
)

// ReadLoop reads WebSocket text messages, parses them, and sends parsed
// messages on incomingMsgCh.
//
// Closes incomingMsgCh on exit.
func ReadLoop(ctx context.Context, ccf context.CancelCauseFunc, doneCh <-chan struct{}, incomingMsgCh chan<- WsMsg, ws *websocket.Conn) {
	var typ websocket.MessageType
	var txt []byte
	var err error
	defer close(incomingMsgCh)
	for err == nil {
		if typ, txt, err = ws.Read(ctx); typ == websocket.MessageText {
			if msg, ok := Parse(txt); ok {
				select {
				case <-ctx.Done():
					return
				case <-doneCh:
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

// WriteLoop reads messages from outboundMsgCh, formats them, and writes them
// to the WebSocket.
//
// Closes the WebSocket on exit.
func WriteLoop(ctx context.Context, ccf context.CancelCauseFunc, doneCh <-chan struct{}, outboundMsgCh <-chan WsMsg, ws *websocket.Conn) {
	defer ws.Close(websocket.StatusNormalClosure, "")
	var err error
	for err == nil {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			return
		case msg, ok := <-outboundMsgCh:
			if !ok {
				return
			}
			var wc io.WriteCloser
			if wc, err = ws.Writer(ctx, websocket.MessageText); err == nil {
				err = writeData(wc, msg, outboundMsgCh)
			}
		}
	}
	if ccf != nil {
		ccf(err)
	}
}

// PingLoop sends periodic WebSocket pings and reports ping errors through ccf.
//
// Returns immediately when interval is non-positive.
func PingLoop(ctx context.Context, ccf context.CancelCauseFunc, doneCh <-chan struct{}, interval, timeout time.Duration, ws *websocket.Conn) {
	if interval > 0 {
		t := time.NewTicker(interval)
		defer t.Stop()

		var err error
		for err == nil {
			select {
			case <-ctx.Done():
				return
			case <-doneCh:
				return
			case <-t.C:
				pingctx, cancel := context.WithTimeout(ctx, timeout)
				err = ws.Ping(pingctx)
				cancel()
			}
		}
		if ccf != nil {
			ccf(err)
		}
	}
}

func writeData(wc io.WriteCloser, firstMsg WsMsg, outboundMsgCh <-chan WsMsg) (err error) {
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
