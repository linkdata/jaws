package wire

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

func Benchmark_wsMsg_AppendAlert(b *testing.B) {
	m := WsMsg{
		Data: "name",
		Jid:  1,
		What: what.Alert,
	}
	for i := 0; i < b.N; i++ {
		m.Append(nil)
	}
}

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
			name: "pointer data with coordinates, modifiers and buttons",
			fields: fields{
				Data: "move 10.5 20.25 5 -1 1 name",
				Jid:  1,
				What: what.Pointer,
			},
			want: "Pointer\tJid.1\t\"move 10.5 20.25 5 -1 1 name\"\n",
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
		{"unquoted", "Input\tJid.1\ttrue\n", WsMsg{Jid: jid.Jid(1), What: what.Input, Data: "true"}},
		{"normal", "Input\tJid.2\t\"c\"\n", WsMsg{Jid: jid.Jid(2), What: what.Input, Data: "c"}},
		{"context menu", "ContextMenu\tJid.2\t\"1 2 5 name\"\n", WsMsg{Jid: jid.Jid(2), What: what.ContextMenu, Data: "1 2 5 name"}},
		{"pointer", "Pointer\tJid.2\t\"move 1.5 2.25 5 -1 1 name\"\n", WsMsg{Jid: jid.Jid(2), What: what.Pointer, Data: "move 1.5 2.25 5 -1 1 name"}},
		{"newline", "Input\tJid.3\t\"c\\nd\"\n", WsMsg{Jid: jid.Jid(3), What: what.Input, Data: "c\nd"}},
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

func Fuzz_wsParse(f *testing.F) {
	f.Add([]byte("Update\t\t\"\"\n"))
	f.Add([]byte("Click\t\t\"10 20 5 name\\tJid.1\"\n"))
	f.Add([]byte("ContextMenu\tJid.1\t\"1 2 0 menu\"\n"))
	f.Add([]byte("Pointer\tJid.1\t\"move 1.5 2.25 5 -1 1 draw\"\n"))
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
	f.Add(uint8(what.Pointer), int32(2), "move 1.5 2.25 5 -1 1 draw")
	f.Add(uint8(what.Set), int32(3), `path={"a":1}`)
	f.Add(uint8(what.Call), int32(4), `fn=[1,2]`)
	f.Fuzz(func(t *testing.T, whatv uint8, jidv int32, data string) {
		wht := what.What(whatv)
		id := jid.Jid(jidv)
		if !wht.IsValid() || !id.IsValid() || id < 0 {
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
