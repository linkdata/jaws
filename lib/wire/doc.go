// Package wire formats and parses the line-based JaWS WebSocket protocol.
//
// The package has two message layers. [Message] is an in-process dispatch record
// routed through [Message.Dest]. [WsMsg] is one browser protocol record;
// [WsMsg.Append] serializes it and [Parse] recovers it.
//
// [ReadLoop] and [WriteLoop] deliver records over a WebSocket. See
// [github.com/linkdata/jaws/lib/what] for commands and events, and
// [github.com/linkdata/jaws/lib/tag] for destination tags.
package wire
