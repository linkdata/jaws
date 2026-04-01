package named

import (
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/jid"
)

// WriteHTMLSelect writes a select tag with options from a NamedBoolArray.
func WriteHTMLSelect(w io.Writer, jid jid.Jid, nba *BoolArray, attrs []template.HTMLAttr) (err error) {
	if err = htmlio.WriteHTMLTag(w, jid, "select", "", "", attrs); err == nil {
		var b []byte
		nba.ReadLocked(func(nba []*Bool) {
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
