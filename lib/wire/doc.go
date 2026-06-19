// Package wire formats and parses the line-based JaWS WebSocket protocol.
//
// Each message is encoded as What<TAB>Jid<TAB>Data<LF>. Data for most commands is
// written by [WsMsg.Append] as a JSON-compatible quoted string so the browser can
// decode it with JSON.parse. [Parse] decodes quoted inbound data with
// strconv.Unquote for the common case, falls back to JSON string decoding for
// browser-valid strings that strconv rejects, and sanitizes the result as valid
// UTF-8. [AppendJSONQuote] stays inside the overlap between those string grammars
// so server-generated frames round-trip through either decoder.
//
// The Set and Call commands carry path/function payloads directly, so callers
// must keep those payloads free of raw tabs and newlines. The path/function side
// also uses '=' as its separator from the JSON value; jaws.JsCall normalizes its
// function path and compacts or escapes JSON before the payload enters the wire
// layer.
package wire
