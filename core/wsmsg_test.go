package core

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/jaws/what"
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
		Jid  Jid
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
			name: "text data",
			fields: fields{
				Data: "name",
				Jid:  1,
				What: what.Click,
			},
			want: "Click\tJid.1\t\"name\"\n",
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
				m2, ok := wsParse([]byte(got))
				if !ok || !reflect.DeepEqual(m, m2) {
					t.Errorf("wsParse(%q) = %v, %v want %v", got, m2, ok, m)
				}
			}
		})
	}
}

func Test_wsParse_CompletePasses(t *testing.T) {
	tests := []struct {
		name string
		txt  string
		want WsMsg
	}{
		{"shortest", "Update\t\t\n", WsMsg{What: what.Update}},
		{"unquoted", "Input\tJid.1\ttrue\n", WsMsg{Jid: Jid(1), What: what.Input, Data: "true"}},
		{"normal", "Input\tJid.2\t\"c\"\n", WsMsg{Jid: Jid(2), What: what.Input, Data: "c"}},
		{"newline", "Input\tJid.3\t\"c\\nd\"\n", WsMsg{Jid: Jid(3), What: what.Input, Data: "c\nd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := wsParse([]byte(tt.txt))
			if !ok || tt.want != got {
				t.Errorf("wsParse(%q): got %q want %q", tt.txt, got, tt.want)
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
			got, ok := wsParse(tt.txt)
			if ok || !reflect.DeepEqual(got, WsMsg{}) {
				t.Errorf("wsParse(%q): got %q wanted wsMsg{}", tt.txt, got)
			}
		})
	}
}

func Fuzz_wsParse(f *testing.F) {
	f.Add([]byte("Update\t\t\"\"\n"))
	f.Add([]byte("Click\t\t\" \\n\"\n"))
	f.Add([]byte("Inner\tJid.1\t\"data\\nline\"\n"))
	f.Fuzz(func(t *testing.T, a []byte) {
		if msg, ok := wsParse(a); ok {
			b := msg.Append(nil)
			if !bytes.Equal(a, b) {
				t.Errorf("%q != %q", string(a), string(b))
			}
		} else {
			if len(a) > 0 && a[0] != '\n' && strings.Count(string(a), "\n") > 1 {
				t.Errorf("%q => nil", a)
			}
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
		{"ErrEventUnhandled", ErrEventUnhandled, "Alert\t\t\"danger\\nevent unhandled\"\n"},
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
