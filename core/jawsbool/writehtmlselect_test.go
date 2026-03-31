package jawsbool

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

func TestWriteHTMLSelect(t *testing.T) {
	nba := NewNamedBoolArray(false)
	nba.Add("one", "1")
	nba.Add("two", "2")
	nba.Set("two", true)

	var sb strings.Builder
	err := WriteHTMLSelect(&sb, jid.Jid(7), nba, []template.HTMLAttr{"", `data-role="sample"`})
	if err != nil {
		t.Fatal(err)
	}

	want := `<select id="Jid.7" data-role="sample">
<option value="one">1</option>
<option value="two" selected>2</option>
</select>`
	if got := sb.String(); got != want {
		t.Fatalf("WriteHTMLSelect():\n got = %q\nwant = %q", got, want)
	}
}

func TestWriteHTMLSelect_Error(t *testing.T) {
	nba := NewNamedBoolArray(false).Add("one", "1")
	wantErr := errors.New("boom")
	err := WriteHTMLSelect(errWriteHTMLSelectWriter{err: wantErr}, jid.Jid(1), nba, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}
