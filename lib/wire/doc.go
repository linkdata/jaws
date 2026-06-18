// Package wire formats and parses the line-based JaWS WebSocket protocol.
//
// Each message is encoded as What<TAB>Jid<TAB>Data<LF>. Data for most commands is
// written by WsMsg.Append as a JSON-compatible quoted string (see appendJSONQuote)
// so the browser can decode it with JSON.parse; the server decodes it with Parse
// (strconv.Unquote, which accepts every escape appendJSONQuote emits). The string
// grammars of JSON and strconv.Unquote merely overlap rather than nest, but
// appendJSONQuote stays inside their intersection. The Set and Call commands carry
// path/function payloads directly, so callers must keep those payloads free of raw
// tabs and newlines. The path/function side also uses '=' as its separator from
// the JSON value; jaws.JsCall normalizes its function path and compacts or escapes
// JSON before the payload enters the wire layer.
package wire
