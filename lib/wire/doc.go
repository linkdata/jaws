// Package wire formats and parses the line-based JaWS WebSocket protocol.
//
// Each message is encoded as What<TAB>Jid<TAB>Data<LF>. Data for most commands
// is JSON-quoted by WsMsg.Append and unquoted by Parse. The Set and Call
// commands carry path/function payloads directly, so callers must keep those
// payloads free of raw tabs and newlines; jaws.JsCall compacts JSON for that
// reason before it enters the wire layer.
package wire
