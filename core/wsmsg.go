package jaws

import "github.com/linkdata/jaws/core/wire"

// WsMsg is a message sent to or from a WebSocket.
type WsMsg = wire.WsMsg

// wsParse parses an incoming text buffer into a message.
func wsParse(txt []byte) (WsMsg, bool) {
	return wire.Parse(txt)
}
