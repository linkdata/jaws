package wire

import (
	"bytes"
	"html"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

const hexDigits = "0123456789abcdef"

// appendJSONQuote appends s to b as a double-quoted JSON string literal.
//
// Unlike strconv.AppendQuote (Go string-literal grammar) it emits only escapes
// that the browser's JSON.parse accepts: it never produces \a, \v, \xNN or
// \UXXXXXXXX. Control bytes use \uXXXX, " and \ are escaped, and everything else
// (including '<', '>', '&' and astral runes) is written as literal UTF-8 to keep
// payloads compact. Invalid UTF-8 is replaced with U+FFFD so the result is always
// valid JSON and a valid WebSocket text frame. The output remains decodable by
// strconv.Unquote, so the server-side Append->Parse round trip is preserved.
//
// PROVISIONAL: this hand-rolled quoter exists only because the stable standard
// library has no zero-allocation "append a non-HTML-escaped JSON string to a
// []byte" primitive: encoding/json.Marshal HTML-escapes '<', '>' and '&' (which
// bloats the HTML payloads this protocol carries), and json.Encoder with
// SetEscapeHTML(false) needs a buffer and is not an append API. The exact
// primitive, jsontext.AppendQuote (encoding/json/v2), is gated behind
// GOEXPERIMENT=jsonv2 as of Go 1.26. Replace this with jsontext.AppendQuote once
// that package builds without the experiment; Fuzz_appendJSONQuote pins the
// behavior to the standard library until then.
func appendJSONQuote(b []byte, s string) []byte {
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if r < 0x20 {
				b = append(b, '\\', 'u', '0', '0', hexDigits[byte(r)>>4], hexDigits[byte(r)&0x0f])
			} else {
				b = utf8.AppendRune(b, r)
			}
		}
	}
	return append(b, '"')
}

// WsMsg is a message sent to or from a WebSocket.
type WsMsg struct {
	Data string    // data to send
	Jid  jid.Jid   // Jid to send, or -1 if Data contains that already
	What what.What // command
}

// Append appends m in wire format to b and returns the extended buffer.
func (m *WsMsg) Append(b []byte) []byte {
	b = append(b, m.What.String()...)
	b = append(b, '\t')
	if m.Jid >= 0 {
		if m.Jid > 0 {
			b = m.Jid.Append(b)
		}
		b = append(b, '\t')
		switch m.What {
		case what.Set, what.Call:
			b = append(b, m.Data...)
		default:
			b = appendJSONQuote(b, m.Data)
		}
	} else {
		b = append(b, m.Data...)
	}
	b = append(b, '\n')
	return b
}

// Format returns m in wire format.
func (m *WsMsg) Format() string {
	return string(m.Append(nil))
}

// Parse parses an incoming text buffer into a message.
//
// The wire format mirrors [WsMsg.Append]. For commands other than [what.Set] and
// [what.Call], if the Data field begins with a double quote it is decoded with
// [strconv.Unquote] and the message is rejected if that fails; data that does not
// begin with a double quote is taken verbatim. Set and Call data is always taken
// verbatim. In all cases the resulting data is sanitized with
// [strings.ToValidUTF8].
func Parse(txt []byte) (WsMsg, bool) {
	if len(txt) > 2 && txt[len(txt)-1] == '\n' {
		if nl1 := bytes.IndexByte(txt, '\t'); nl1 >= 0 {
			if nl2 := bytes.IndexByte(txt[nl1+1:], '\t'); nl2 >= 0 {
				nl2 += nl1 + 1
				// What       ... Jid              ... Data                  ... EOL
				// txt[0:nl1] ... txt[nl1+1 : nl2] ... txt[nl2+1:len(txt)-1] ... \n
				if wht := what.Parse(string(txt[0:nl1])); wht.IsValid() {
					if id := jid.ParseString(string(txt[nl1+1 : nl2])); id.IsValid() {
						data := string(txt[nl2+1 : len(txt)-1])
						if txt[nl2+1] == '"' && wht != what.Set && wht != what.Call {
							var err error
							if data, err = strconv.Unquote(data); err != nil {
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

// FillAlert replaces m with an escaped danger alert for err.
func (m *WsMsg) FillAlert(err error) {
	m.Jid = 0
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
