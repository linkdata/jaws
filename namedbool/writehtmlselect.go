package namedbool

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/jawshtml"
	"github.com/linkdata/jaws/jid"
)

// WriteHTMLSelect writes a select tag with options from a NamedBoolArray.
func WriteHTMLSelect(w io.Writer, jid jid.Jid, nba *NamedBoolArray, attrs []template.HTMLAttr) (err error) {
	if err = jawshtml.WriteHTMLTag(w, jid, "select", "", "", attrs); err == nil {
		var b []byte
		nba.ReadLocked(func(nba []*NamedBool) {
			for _, nb := range nba {
				b = append(b, "\n<option value="...)
				b = strconv.AppendQuote(b, nb.Name())
				if nb.Checked() {
					b = append(b, ` selected`...)
				}
				b = append(b, '>')
				b = append(b, nb.HTML()...)
				b = append(b, "</option>"...)
			}
		})
		b = append(b, "\n</select>"...)
		_, err = w.Write(b)
	}
	return
}
