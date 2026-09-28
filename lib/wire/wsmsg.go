package wire

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"html"
	"strconv"
	"strings"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

// AppendJSONQuote appends s to b as a JSON string literal accepted by JSON.parse.
//
// Invalid UTF-8 is replaced with U+FFFD. Unlike [strconv.AppendQuote], the
// result uses JSON escapes and does not HTML-escape the data.
func AppendJSONQuote(b []byte, s string) []byte {
	// Invalid UTF-8 still produces valid output with replacement runes.
	quoted, _ := jsontext.AppendQuote(b, s)
	return quoted
}

// WsMsg is a protocol record sent to or from a WebSocket.
type WsMsg struct {
	Data string    // data to send
	Jid  jid.Jid   // non-negative Jid to send
	What what.What // command
}

// Append appends m in wire format to b and returns the extended buffer.
//
// The record is What<TAB>Jid<TAB>Data<LF>, where the Jid field is empty if Jid
// is zero. The Data field is written verbatim for [what.JsVar] and [what.Call],
// and JSON-quoted for every other command. Append panics if Jid is negative.
//
// Verbatim Data must contain no tab or newline bytes, which would corrupt the
// record; ensuring that is the caller's responsibility.
func (m *WsMsg) Append(b []byte) []byte {
	if m.Jid < 0 {
		panic("wire.WsMsg.Append: negative Jid")
	}
	b = append(b, m.What.String()...)
	b = append(b, '\t')
	if m.Jid > 0 {
		b = m.Jid.Append(b)
	}
	b = append(b, '\t')
	switch m.What {
	case what.JsVar, what.Call:
		b = append(b, m.Data...)
	default:
		b = AppendJSONQuote(b, m.Data)
	}
	b = append(b, '\n')
	return b
}

// Format returns m in wire format.
//
// Format panics if Jid is negative.
func (m *WsMsg) Format() string {
	return string(m.Append(nil))
}

// Parse parses one LF-terminated protocol record.
//
// The wire format mirrors [WsMsg.Append]. For commands other than [what.JsVar]
// and [what.Call], if the Data field begins with a double quote
// it is decoded as a JSON string: [strconv.Unquote] handles the common case,
// with a fallback to a JSON
// string decode for inputs it rejects but the browser's JSON.stringify can produce
// (notably a lone UTF-16 surrogate, which the fallback maps to U+FFFD). The message
// is rejected only if both decoders fail. Data that does not begin with a double
// quote is taken verbatim, as is all JsVar and Call data. In all cases the resulting
// data is sanitized with [strings.ToValidUTF8].
//
// Inbound [what.JsVar] and [what.Call] data is taken verbatim at the field
// boundaries and is best-effort: the field ends at the first tab, so a tab
// inside an inbound JsVar or Call payload truncates the field.
func Parse(txt []byte) (WsMsg, bool) {
	// Parse reports success with ok rather than an error: the only failure is "txt is
	// not a valid record", with no sub-cause any caller branches on, and the sole caller
	// (ReadLoop) drops an unparseable record without inspecting why — so a single
	// always-equal error would carry nothing the bool does not.
	//
	// The len(txt) > 2 floor keeps the trailing-newline check and the two tab scans
	// below from indexing out of range; a structurally minimal record is What\t\t\n, and
	// any shorter or otherwise malformed input is rejected by the tab-field checks.
	//
	// Parse requires the two-tab What\tJid\tData form emitted by Append.
	if len(txt) > 2 && txt[len(txt)-1] == '\n' {
		if nl1 := bytes.IndexByte(txt, '\t'); nl1 >= 0 {
			if nl2 := bytes.IndexByte(txt[nl1+1:], '\t'); nl2 >= 0 {
				nl2 += nl1 + 1
				// What       ... Jid              ... Data                  ... EOL
				// txt[0:nl1] ... txt[nl1+1 : nl2] ... txt[nl2+1:len(txt)-1] ... \n
				if wht := what.Parse(string(txt[0:nl1])); wht.IsValid() {
					if id := jid.ParseString(string(txt[nl1+1 : nl2])); id.IsValid() {
						raw := txt[nl2+1 : len(txt)-1]
						if wht == what.JsVar || wht == what.Call {
							// JsVar and Call data is taken verbatim and is best-effort:
							// the field ends at the first tab, so drop any tab-separated
							// suffix an untrusted record appended past that boundary.
							if i := bytes.IndexByte(raw, '\t'); i >= 0 {
								raw = raw[:i]
							}
						}
						data := string(raw)
						if txt[nl2+1] == '"' && wht != what.JsVar && wht != what.Call {
							// The browser encodes this data with JSON.stringify.
							// strconv.Unquote decodes the common case cheaply and
							// allocation-free, but its grammar is not a superset of
							// JSON: it rejects the "\udXXX" lone-surrogate escapes
							// JSON.stringify can emit. Fall back to a JSON string
							// decode (which maps a lone surrogate to U+FFFD) so a
							// legitimate event is decoded rather than silently dropped;
							// the ToValidUTF8 below still sanitizes whatever survives.
							// The fallback lives in jsonUnquoteString so the address it
							// takes does not force data to the heap on every call.
							if unq, err := strconv.Unquote(data); err == nil {
								data = unq
							} else if unq, ok := jsonUnquoteString(data); ok {
								data = unq
							} else {
								return WsMsg{}, false
							}
						}
						return WsMsg{
							Data: strings.ToValidUTF8(data, ""),
							Jid:  id,
							What: wht,
						}, true
					}
				}
			}
		}
	}
	return WsMsg{}, false
}

// jsonUnquoteString decodes s as a JSON string literal, returning ok=false if it
// is not one. It exists as a separate function so that the address it must take of
// its decode target does not force [Parse]'s data local to escape to the heap on
// every call; see the fallback in Parse.
func jsonUnquoteString(s string) (out string, ok bool) {
	ok = json.Unmarshal([]byte(s), &out) == nil
	return
}

// FillAlert replaces m with an escaped danger alert for err.
//
// A nil err yields a danger alert with an empty message rather than panicking.
func (m *WsMsg) FillAlert(err error) {
	var s string
	if err != nil {
		s = err.Error()
	}
	m.Jid = 0
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(s)
}
