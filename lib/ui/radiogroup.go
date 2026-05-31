package ui

import (
	"html/template"
	"strings"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// RadioElement renders the input and label elements for one radio option.
//
// The underlying [jaws.Element] values are created lazily on the first call to
// [RadioElement.Radio] or [RadioElement.Label], so options that a template never
// renders register no elements on the [jaws.Request]. Call each of Radio and
// Label at most once.
type RadioElement struct {
	st *radioState
}

// radioState is the shared, lazily-populated state behind a RadioElement. It is
// held by pointer so the value-receiver methods (callable on a template range
// copy) observe the same elements, and is only ever touched on the single
// rendering goroutine, so it needs no lock.
type radioState struct {
	rw       RequestWriter
	nb       *named.Bool
	nameAttr string
	radio    *jaws.Element
	label    *jaws.Element
}

// radioElem returns the radio Element, creating it on first use. Label also
// needs it (for the "for=" attribute), so creating it here keeps the radio's Jid
// ordered before the label's regardless of which is rendered first.
func (st *radioState) radioElem() *jaws.Element {
	if st.radio == nil {
		st.radio = st.rw.Request.NewElement(NewRadio(st.nb))
	}
	return st.radio
}

// Radio renders an HTML input element of type radio.
func (re RadioElement) Radio(params ...any) template.HTML {
	radio := re.st.radioElem()
	var sb strings.Builder
	radio.Jaws.MustLog(radio.JawsRender(&sb, append(params, re.st.nameAttr)))
	return template.HTML(sb.String()) // #nosec G203
}

// Label renders an HTML label element.
func (re RadioElement) Label(params ...any) template.HTML {
	radio := re.st.radioElem()
	if re.st.label == nil {
		re.st.label = re.st.rw.Request.NewElement(NewLabel(re.st.nb))
	}
	var sb strings.Builder
	forAttr := string(radio.Jid().AppendQuote([]byte("for=")))
	re.st.label.Jaws.MustLog(re.st.label.JawsRender(&sb, append(params, forAttr)))
	return template.HTML(sb.String()) // #nosec G203
}

// RadioGroup returns a [RadioElement] for each value in nba. Elements are created
// lazily as they are rendered; see [RadioElement].
func (rw RequestWriter) RadioGroup(nba *named.BoolArray) (rel []RadioElement) {
	nameAttr := `name="` + jaws.MakeID() + `"`
	nba.ReadLocked(func(nbl []*named.Bool) {
		for _, nb := range nbl {
			rel = append(rel, RadioElement{st: &radioState{
				rw:       rw,
				nb:       nb,
				nameAttr: nameAttr,
			}})
		}
	})
	return
}
