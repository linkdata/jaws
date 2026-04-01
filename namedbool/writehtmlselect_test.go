package namedbool

import (
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/jid"
)

type errWriteHTMLSelectWriter struct {
	err error
}

func (w errWriteHTMLSelectWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriteHTMLSelect_Error(t *testing.T) {
	nba := NewArray(false).Add("one", "1")
	wantErr := errors.New("boom")
	err := WriteHTMLSelect(errWriteHTMLSelectWriter{err: wantErr}, jid.Jid(1), nba, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func Test_WriteHTMLSelect(t *testing.T) {
	type args struct {
		jid   jid.Jid
		val   *NamedBoolArray
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLSelect empty NamedBoolArray and one attr",
			args: args{
				jid:   2,
				val:   NewArray(false),
				attrs: []template.HTMLAttr{"attr1"},
			},
			want: "<select id=\"Jid.2\" attr1>\n</select>",
		},
		{
			name: "HTMLSelect NamedBoolArray with data",
			args: args{
				jid: 3,
				val: func() (nba *NamedBoolArray) {
					nba = NewArray(false)
					nba.Add("one", "1")
					nba.Add("two", "2")
					nba.Add("three", "2")
					nba.Set("two", true)
					return
				}(),
				attrs: []template.HTMLAttr{"", "attr2"},
			},
			want: `<select id="Jid.3" attr2>
<option value="one">1</option>
<option value="two" selected>2</option>
<option value="three">2</option>
</select>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := WriteHTMLSelect(&sb, tt.args.jid, tt.args.val, tt.args.attrs); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLSelect():\n got = %q\nwant = %q", got, tt.want)
			}
		})
	}
}
