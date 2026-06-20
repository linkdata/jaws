package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

func Test_wsMsg_Append(t *testing.T) {
	type fields struct {
		Data string
		Jid  jid.Jid
		What what.What
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		noparse bool
	}{
		{
			name:    "empty",
			fields:  fields{},
			want:    "invalid\t\t\"\"\n",
			noparse: true,
		},
		{
			name: "zero Jid",
			fields: fields{
				Data: "",
				Jid:  0,
				What: what.Update,
			},
			want: "Update\t\t\"\"\n",
		},
		{
			name: "click data with coordinates and modifiers",
			fields: fields{
				Data: "10 20 5 name",
				Jid:  1,
				What: what.Click,
			},
			want: "Click\tJid.1\t\"10 20 5 name\"\n",
		},
		{
			name: "context menu data with coordinates and modifiers",
			fields: fields{
				Data: "10 20 5 name",
				Jid:  1,
				What: what.ContextMenu,
			},
			want: "ContextMenu\tJid.1\t\"10 20 5 name\"\n",
		},
		{
			name: "escaped text data",
			fields: fields{
				Data: "double\"quote",
				Jid:  1,
				What: what.Click,
			},
			want: "Click\tJid.1\t\"double\\\"quote\"\n",
		},
		{
			name: "pass-through data",
			fields: fields{
				Data: "custom_id\t\"text\"",
				Jid:  -1,
				What: what.Click,
			},
			want:    "Click\tcustom_id\t\"text\"\n",
			noparse: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := WsMsg{
				Data: tt.fields.Data,
				Jid:  tt.fields.Jid,
				What: tt.fields.What,
			}
			if got := string(m.Append(nil)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wsMsg.Append() = %q, want %q", got, tt.want)
			} else if !tt.noparse {
				m2, ok := Parse([]byte(got))
				if !ok || !reflect.DeepEqual(m, m2) {
					t.Errorf("Parse(%q) = %v, %v want %v", got, m2, ok, m)
				}
			}
		})
	}
}

func Test_wsMsg_AppendNegativeJidAndCallPayload(t *testing.T) {
	msg := WsMsg{Jid: jid.Jid(-1), What: what.Update, Data: "raw\tdata"}
	if got := string(msg.Append(nil)); got != "Update\traw\tdata\n" {
		t.Fatalf("unexpected ws append result %q", got)
	}

	msg = WsMsg{Jid: 1, What: what.Call, Data: `fn={"a":1}`}
	if got := string(msg.Append(nil)); !strings.Contains(got, `fn={"a":1}`) || strings.Contains(got, `"fn={"`) {
		t.Fatalf("unexpected ws append quoted call payload %q", got)
	}
}

func Test_wsParse_CompletePasses(t *testing.T) {
	tests := []struct {
		name string
		txt  string
		want WsMsg
	}{
		{"shortest", "Update\t\t\n", WsMsg{What: what.Update}},
		// An empty What field parses as what.Update, because what.Parse("") returns
		// Update. Pins that empty-command frames are accepted, not rejected.
		{"empty What is Update", "\t\t\n", WsMsg{What: what.Update}},
		{"unquoted", "Input\tJid.1\ttrue\n", WsMsg{Jid: jid.Jid(1), What: what.Input, Data: "true"}},
		{"normal", "Input\tJid.2\t\"c\"\n", WsMsg{Jid: jid.Jid(2), What: what.Input, Data: "c"}},
		{"context menu", "ContextMenu\tJid.2\t\"1 2 5 name\"\n", WsMsg{Jid: jid.Jid(2), What: what.ContextMenu, Data: "1 2 5 name"}},
		{"newline", "Input\tJid.3\t\"c\\nd\"\n", WsMsg{Jid: jid.Jid(3), What: what.Input, Data: "c\nd"}},
		// Set and Call data is taken verbatim even when it begins with a double
		// quote: it must not be run through strconv.Unquote (Parse excludes Set/Call
		// from unquoting). Pins that guard against accidental removal.
		{"set quote-prefixed verbatim", "Set\tJid.4\t\"x\"=1\n", WsMsg{Jid: jid.Jid(4), What: what.Set, Data: "\"x\"=1"}},
		{"call quote-prefixed verbatim", "Call\tJid.5\t\"x\"=1\n", WsMsg{Jid: jid.Jid(5), What: what.Call, Data: "\"x\"=1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse([]byte(tt.txt))
			if !ok || tt.want != got {
				t.Errorf("Parse(%q): got %q want %q", tt.txt, got, tt.want)
			}
		})
	}
}

func Test_wsParse_IncompleteFails(t *testing.T) {
	tests := []struct {
		name string
		txt  []byte
	}{
		{"nil", nil},
		{"invalid What", []byte("invalid\t\t\n")},
		{"missing ending linefeed", []byte("Click\t\t")},
		{"just one tab", []byte("Click\t\n")},
		{"newline instead of What", []byte("\n\t\t\n")},
		{"incorrectly quoted data", []byte("Click\t\t\"\n\"\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse(tt.txt)
			if ok || !reflect.DeepEqual(got, WsMsg{}) {
				t.Errorf("Parse(%q): got %q wanted wsMsg{}", tt.txt, got)
			}
		})
	}
}

// Test_wsParse_InboundLoneSurrogate covers an inbound frame whose JSON-quoted data
// holds a lone UTF-16 surrogate. The browser's JSON.stringify emits such a value as
// the literal escape "\udXXX" for Input/Click/ContextMenu/Remove payloads (it does
// not throw on lone surrogates), but strconv.Unquote rejects "\udXXX", which would
// silently drop the whole event frame before the ToValidUTF8 sanitizer runs. Parse
// must instead decode it, replacing the surrogate with U+FFFD, and deliver the event.
func Test_wsParse_InboundLoneSurrogate(t *testing.T) {
	frame := "Input\tJid.1\t\"\\ud800\"\n" // data field is the 8-byte JSON string "\ud800"
	msg, ok := Parse([]byte(frame))
	if !ok {
		t.Fatalf("Parse dropped a frame with a lone surrogate: %q", frame)
	}
	if msg.What != what.Input || msg.Jid != jid.Jid(1) {
		t.Errorf("unexpected header: %+v", msg)
	}
	if !utf8.ValidString(msg.Data) {
		t.Errorf("Data is not valid UTF-8: %q", msg.Data)
	}
	if msg.Data != "�" {
		t.Errorf("got Data %q, want the replacement char %q", msg.Data, "�")
	}
}

func Fuzz_wsParse(f *testing.F) {
	f.Add([]byte("Update\t\t\"\"\n"))
	f.Add([]byte("Click\t\t\"10 20 5 name\\tJid.1\"\n"))
	f.Add([]byte("ContextMenu\tJid.1\t\"1 2 0 menu\"\n"))
	f.Add([]byte("Inner\tJid.1\t\"data\\nline\"\n"))
	f.Add([]byte("Set\tJid.1\tpath={\"a\":1}\n"))
	f.Add([]byte("Call\tJid.1\tfn=[1,2]\n"))
	f.Add([]byte("invalid\t\t\"\"\n"))
	f.Fuzz(func(t *testing.T, a []byte) {
		msg, ok := Parse(a)
		if !ok {
			return
		}
		b := msg.Append(nil)
		if len(b) == 0 || b[len(b)-1] != '\n' {
			t.Fatalf("Append did not produce newline-terminated frame: %q", b)
		}
		msg2, ok := Parse(b)
		if !ok {
			t.Fatalf("Parse(Append(Parse(x))) failed: in=%q appended=%q msg=%+v", a, b, msg)
		}
		if msg != msg2 {
			t.Fatalf("roundtrip mismatch: in=%q parsed=%+v reparsed=%+v", a, msg, msg2)
		}
	})
}

func Fuzz_wsMsgAppendParseRoundTrip(f *testing.F) {
	f.Add(uint8(what.Input), int32(0), "value")
	f.Add(uint8(what.Click), int32(1), "1 2 5 name")
	f.Add(uint8(what.ContextMenu), int32(2), "3 4 2 menu")
	f.Add(uint8(what.Set), int32(3), `path={"a":1}`)
	f.Add(uint8(what.Call), int32(4), `fn=[1,2]`)
	f.Fuzz(func(t *testing.T, whatv uint8, jidv int32, data string) {
		wht := what.What(whatv)
		id := jid.Jid(jidv)
		if !wht.IsValid() || !id.IsValid() || id < 0 {
			return
		}
		// Append encodes the command via wht.String() and Parse decodes it via
		// what.Parse; only real named commands round-trip. IsValid merely excludes
		// the two boundary markers, so also require String/Parse to round-trip,
		// which is always the case for What values that actually occur on the wire.
		if what.Parse(wht.String()) != wht {
			return
		}
		// Append/Parse deliberately normalize data to valid UTF-8 (Parse runs
		// strings.ToValidUTF8 and appendJSONQuote replaces invalid runes with
		// U+FFFD), so the round-trip identity only holds for already-valid UTF-8.
		if !utf8.ValidString(data) {
			return
		}
		// Set/Call data is written verbatim, so it must not contain the tab/newline
		// framing delimiters; for other commands appendJSONQuote escapes them.
		if (wht == what.Set || wht == what.Call) && strings.ContainsAny(data, "\t\n") {
			return
		}
		msg := WsMsg{
			Data: data,
			Jid:  id,
			What: wht,
		}
		b := msg.Append(nil)
		got, ok := Parse(b)
		if !ok {
			t.Fatalf("Parse(Append(msg)) failed: msg=%+v frame=%q", msg, b)
		}
		if got != msg {
			t.Fatalf("Parse(Append(msg)) mismatch: want=%+v got=%+v frame=%q", msg, got, b)
		}
	})
}

// Test_wsMsg_AppendDataIsValidJSON guards the wire contract that Append's quoted
// data field is valid JSON, so the browser's JSON.parse can decode it. It exercises
// control bytes and astral runes that must be emitted as JSON-compatible escapes
// (never the Go-only \a, \v, \xNN or \UXXXXXXXX forms that JSON.parse rejects).
func Test_wsMsg_AppendDataIsValidJSON(t *testing.T) {
	for _, data := range []string{
		"plain",
		"tab\tnewline\nreturn\r",
		"bell\x07 vtab\x0b del\x7f",
		"ctrl\x00\x01\x1f",
		"astral \U0001F600 and <b>&amp;</b>",
		"quote\" backslash\\",
		string([]byte{0xff, 0xfe}), // invalid UTF-8
	} {
		m := WsMsg{What: what.Inner, Jid: 1, Data: data}
		b := m.Append(nil)
		parts := bytes.SplitN(b[:len(b)-1], []byte{'\t'}, 3) // What \t Jid \t data
		if len(parts) != 3 {
			t.Fatalf("unexpected frame %q", b)
		}
		if !utf8.Valid(parts[2]) {
			t.Errorf("data field for %q is not valid UTF-8: %q", data, parts[2])
		}
		var s string
		if err := json.Unmarshal(parts[2], &s); err != nil {
			t.Errorf("data field for %q is not valid JSON: %v (frame %q)", data, err, b)
		}
	}
}

// Test_wsParse_SanitizesInvalidUTF8InVerbatimData covers Parse's ToValidUTF8
// sanitization of the verbatim Set/Call data path: invalid UTF-8 from the browser
// must be stripped so downstream consumers never see it.
func Test_wsParse_SanitizesInvalidUTF8InVerbatimData(t *testing.T) {
	raw := append([]byte("Set\tJid.1\tx="), 0xff, 0xfe, 'y', '\n')
	msg, ok := Parse(raw)
	if !ok {
		t.Fatal("expected Parse to succeed")
	}
	if !utf8.ValidString(msg.Data) {
		t.Fatalf("Parse left invalid UTF-8 in data: %q", msg.Data)
	}
	if msg.Data != "x=y" {
		t.Errorf("got %q, want %q", msg.Data, "x=y")
	}
}

// stdlibJSONQuote is the standard-library reference for appendJSONQuote: an
// encoder with HTML escaping disabled, which is the behavior the hand-rolled
// quoter mimics (and which jsontext.AppendQuote would provide directly when
// encoding/json/v2 builds without GOEXPERIMENT=jsonv2).
func stdlibJSONQuote(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil { // marshaling a string never fails
		panic(err)
	}
	b := buf.Bytes()
	return b[:len(b)-1] // drop the trailing newline Encode appends
}

// Fuzz_appendJSONQuote pins the PROVISIONAL hand-rolled appendJSONQuote to the
// standard library so it can be replaced confidently when jsontext.AppendQuote
// becomes available. For any input it must (1) produce valid JSON the browser's
// JSON.parse accepts, (2) decode to exactly what the stdlib non-HTML-escaping
// encoder decodes to (cosmetic escape differences are allowed, semantic ones are
// not), and (3) stay decodable by strconv.Unquote so the server-side Append->Parse
// round trip is preserved.
func Fuzz_appendJSONQuote(f *testing.F) {
	for _, s := range []string{
		"",
		"plain",
		"<div>&amp;</div>",
		"a\tb\nc\rd",
		"bell\x07 vtab\x0b back\x08 form\x0c ctrl\x00\x1f",
		"astral \U0001F600 sep    del\x7f",
		"quote\" backslash\\",
		string([]byte{0xff, 0xfe, 0x41}),
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := appendJSONQuote(nil, s)

		if !json.Valid(out) {
			t.Fatalf("output is not valid JSON: in=%q out=%q", s, out)
		}

		var got, want string
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("output does not Unmarshal: in=%q out=%q err=%v", s, out, err)
		}
		if err := json.Unmarshal(stdlibJSONQuote(s), &want); err != nil {
			t.Fatalf("reference does not Unmarshal: in=%q err=%v", s, err)
		}
		if got != want {
			t.Fatalf("decode mismatch vs stdlib: in=%q got=%q want=%q out=%q", s, got, want, out)
		}

		if _, err := strconv.Unquote(string(out)); err != nil {
			t.Fatalf("strconv.Unquote rejects output (Parse would too): in=%q out=%q err=%v", s, out, err)
		}
	})
}

// Test_AppendJSONQuote covers the exported wrapper: it must delegate to
// appendJSONQuote (whose exact behavior Fuzz_appendJSONQuote pins) and honor the
// append contract by extending the supplied buffer rather than replacing it.
func Test_AppendJSONQuote(t *testing.T) {
	for _, s := range []string{
		"",
		"plain",
		"<script>&amp;</script>",
		"tab\tnl\nret\r ctrl\x00\x1f del\x7f",
		"quote\" backslash\\ astral \U0001F600",
		string([]byte{0xff, 0xfe, 0x41}), // invalid UTF-8
	} {
		prefix := []byte("PFX")
		got := AppendJSONQuote(prefix, s)

		// The exported wrapper must append to the buffer, preserving its prefix.
		if !bytes.HasPrefix(got, prefix) {
			t.Errorf("AppendJSONQuote dropped the buffer prefix for %q: %q", s, got)
		}
		quoted := got[len(prefix):]

		// It must produce exactly what the unexported implementation it wraps does.
		if want := appendJSONQuote(nil, s); !bytes.Equal(quoted, want) {
			t.Errorf("AppendJSONQuote(%q) = %q, want %q", s, quoted, want)
		}

		// And that output must be valid JSON the browser's JSON.parse accepts.
		if !json.Valid(quoted) {
			t.Errorf("AppendJSONQuote(%q) is not valid JSON: %q", s, quoted)
		}
	}
}

func Test_wsMsg_FillAlert(t *testing.T) {
	fooError := errors.New("<\"")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"event unhandled", errors.New("event unhandled"), "Alert\t\t\"danger\\nevent unhandled\"\n"},
		{"escape error text", fooError, "Alert\t\t\"danger\\n&lt;&#34;\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m WsMsg
			m.FillAlert(tt.err)
			got := m.Format()
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}
