// Package wire formats and parses the line-based JaWS WebSocket protocol.
//
// The package has two message layers. [Message] is an in-process dispatch record
// routed through [Message.Dest]. [WsMsg] is one browser protocol record;
// [WsMsg.Append] serializes it and [Parse] recovers it.
//
// Each record is What<TAB>Jid<TAB>Data<LF>. One WebSocket text message may contain
// several records; [ReadLoop] preserves valid-record order and skips malformed
// records independently.
//
// [WsMsg.Append] JSON-quotes Data for commands other than
// [github.com/linkdata/jaws/lib/what.Set] and
// [github.com/linkdata/jaws/lib/what.Call]. [Parse] decodes quote-prefixed Data and
// sanitizes every accepted result as valid UTF-8; unquoted Data is accepted
// verbatim. Set and Call always carry verbatim path=json Data, which must contain
// no raw tab or LF delimiters. See [github.com/linkdata/jaws/lib/what] for command
// semantics and [github.com/linkdata/jaws/lib/tag] for destination keys.
package wire
