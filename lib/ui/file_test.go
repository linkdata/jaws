package ui

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/htmlio"
)

func TestFile_Render(t *testing.T) {
	_, rq := newCoreRequest(t)
	tests := []struct {
		name           string
		field          string
		submitOnChange bool
		params         []any
		want           string
	}{
		{
			name:  "manual submit",
			field: "document",
			want:  `^<input id="Jid\.[0-9]+" type="file" name="document">$`,
		},
		{
			name: "empty name",
			want: `^<input id="Jid\.[0-9]+" type="file" name="">$`,
		},
		{
			name:           "submit on change and accept mask",
			field:          "photo",
			submitOnChange: true,
			params:         []any{htmlio.Attr("accept", ".png,.jpg,image/*"), "multiple"},
			want:           `^<input id="Jid\.[0-9]+" type="file" name="photo" data-jawsonchangesubmit accept="\.png,\.jpg,image/\*" multiple>$`,
		},
		{
			name:   "escaped field and mask",
			field:  `a"&<b>`,
			params: []any{htmlio.Attr("accept", `.png,"&<b>`)},
			want:   `^<input id="Jid\.[0-9]+" type="file" name="a&#34;&amp;&lt;b&gt;" accept="\.png,&#34;&amp;&lt;b&gt;">$`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := renderUI(t, rq, NewFile(tt.field, tt.submitOnChange), tt.params...)
			mustMatch(t, tt.want, got)
			if strings.Contains(got, " value=") {
				t.Fatalf("file input has value attribute: %q", got)
			}
		})
	}
}

func TestFile_TemplateHelper(t *testing.T) {
	_, rq := newCoreRequest(t)
	var output bytes.Buffer
	rw := RequestWriter{Request: rq, Writer: &output}
	tmpl := template.Must(template.New("file").Parse(`<form method="post" enctype="multipart/form-data">{{$.File "upload" true "accept=\".pdf\""}}</form>`))
	if err := tmpl.Execute(&output, With{RequestWriter: rw}); err != nil {
		t.Fatal(err)
	}
	mustMatch(t, `^<form method="post" enctype="multipart/form-data"><input id="Jid\.[0-9]+" type="file" name="upload" data-jawsonchangesubmit accept="\.pdf"></form>$`, output.String())
}

func TestFile_UpdateLeavesSelectionAlone(t *testing.T) {
	tr := valueTestNewLoopRequest(t, nil)
	u := NewFile("upload", true)
	elem, _ := renderUI(t, tr.Request, u)
	u.JawsUpdate(elem)
	if got := nestedReorderDrainWire(t, tr, "file update"); len(got) != 0 {
		t.Fatalf("file update commands = %+v, want none", got)
	}
}

func TestFile_RenderWriteError(t *testing.T) {
	_, rq := newCoreRequest(t)
	wantErr := errors.New("write failed")
	elem := rq.NewElement(NewFile("upload", false))
	if err := elem.JawsRender(&failNthWrite{n: 1, err: wantErr}, nil); !errors.Is(err, wantErr) {
		t.Fatalf("JawsRender error = %v, want %v", err, wantErr)
	}
}
