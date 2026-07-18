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
// Label at most once. Render Label only when Radio is also rendered: Label emits a
// for="..." referencing the radio's id, so a Label without its Radio points at an
// input that is absent from the document (and leaves an unrendered radio Element
// registered on the Request for the request's lifetime).
type RadioElement struct {
	st *radioState
}

// radioState is the shared, lazily-populated state behind a RadioElement. It is
// held by pointer so the value-receiver methods (callable on a template range
// copy) observe the same elements, and is only ever touched on the single
// rendering goroutine, so it needs no lock.
type radioState struct {
	rw    RequestWriter
	nb    *named.Bool
	group *radioGroupState
	radio *jaws.Element
	label *jaws.Element
}

// radioGroupState holds the request-scoped name shared by every option in a
// RadioGroup. It is populated from the first lazily-created radio Element's Jid.
type radioGroupState struct {
	nameAttr string
}

// radioElem returns the radio Element, creating it on first use. Label also
// needs it (for the "for=" attribute), so creating it here keeps the radio's Jid
// ordered before the label's regardless of which is rendered first.
func (st *radioState) radioElem() *jaws.Element {
	if st.radio == nil {
		st.radio = st.rw.Request.NewElement(NewRadio(st.nb))
		if st.group.nameAttr == "" {
			st.group.nameAttr = `name="` + st.radio.Jid().String() + `"`
		}
	}
	return st.radio
}

// Radio renders an HTML input element of type radio.
//
// The group's generated name= attribute takes precedence over any name= passed
// in params: it is emitted first and the HTML parser keeps the first of
// duplicate attributes, preserving the invariant that every radio in the group
// shares the same request-scoped name.
//
// Render errors are reported through [jaws.Jaws.MustLog], which panics when
// no [jaws.Jaws.Logger] is configured.
func (re RadioElement) Radio(params ...any) template.HTML {
	radio := re.st.radioElem()
	var sb strings.Builder
	// A fresh slice with nameAttr first avoids mutating the caller's variadic
	// backing array and makes the group name win over any caller-supplied name=.
	radio.Jaws.MustLog(radio.JawsRender(&sb, append([]any{re.st.group.nameAttr}, params...)))
	return template.HTML(sb.String()) // #nosec G203
}

// Label renders an HTML label element.
//
// The generated for= attribute referencing the radio's id takes precedence over
// any for= passed in params: it is emitted first and the HTML parser keeps the
// first of duplicate attributes, so the label always targets its own radio.
//
// Render errors are reported through [jaws.Jaws.MustLog], which panics when
// no [jaws.Jaws.Logger] is configured.
func (re RadioElement) Label(params ...any) template.HTML {
	radio := re.st.radioElem()
	if re.st.label == nil {
		re.st.label = re.st.rw.Request.NewElement(NewLabel(re.st.nb))
	}
	var sb strings.Builder
	forAttr := string(radio.Jid().AppendQuote([]byte("for=")))
	// A fresh slice with forAttr first avoids mutating the caller's variadic
	// backing array and makes the generated for= win over any caller-supplied for=.
	re.st.label.Jaws.MustLog(re.st.label.JawsRender(&sb, append([]any{forAttr}, params...)))
	return template.HTML(sb.String()) // #nosec G203
}

// RadioGroup returns a [RadioElement] for each value in nba.
//
// Elements are created lazily as they are rendered; see [RadioElement]. Every
// rendered radio in the group shares a name derived from the first created
// radio Element's request-scoped [jaws.Jid].
func (rw RequestWriter) RadioGroup(nba *named.BoolArray) (rel []RadioElement) {
	group := &radioGroupState{}
	nba.ReadLocked(func(nbl []*named.Bool) {
		for _, nb := range nbl {
			rel = append(rel, RadioElement{st: &radioState{
				rw:    rw,
				nb:    nb,
				group: group,
			}})
		}
	})
	return
}
