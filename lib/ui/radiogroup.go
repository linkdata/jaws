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
// input that is absent from the document. The radio Element it created is still
// unregistered by the [Template] that owns it (see [RequestWriter.RadioGroup]) when
// that template next replaces its content. With no template owner it has no DOM node
// for a removal to report either, so it stays registered until the [jaws.Request] ends.
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
		// Create and report the Element rather than going through
		// RequestWriter.NewUI: Radio and Label return their HTML for the template to
		// place instead of writing it to the writer, and the Element has to exist
		// before that render because its Jid supplies the group's name= and the
		// label's for=. Reporting it here, at creation, also means a radio that is
		// never rendered (a Label without its Radio) is still owned and reclaimed.
		st.radio = st.rw.Request.NewElement(NewRadio(st.nb))
		st.rw.trackElement(st.radio)
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
		// Created and reported like the radio Element; see radioElem.
		re.st.label = re.st.rw.Request.NewElement(NewLabel(re.st.nb))
		re.st.rw.trackElement(re.st.label)
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
//
// Use RadioGroup with a single-select [named.BoolArray] whose [named.Bool.Name]
// values are distinct (its zero value or [named.NewBoolArray](false)) when Go
// state must follow native single-selection behavior. A multi-select BoolArray
// or duplicate Bool names are incompatible with native radio semantics.
// Separately bound [Radio] widgets do not derive server-side grouping from their
// native HTML group.
//
// The radio and label Elements belong to the [Template] whose body called RadioGroup,
// which unregisters them when it next replaces its content. Ownership follows that call
// site rather than the wrapper the markup lands in, so passing [RadioElement] values
// into a nested wrapped template through its dot leaves them owned by the outer
// template: an update of the inner wrapper alone replaces their DOM without their owner
// reclaiming them, leaving that to the browser's removal acknowledgement for the ids
// that reached the DOM and to the outer template's next update for any that did not.
// Re-rendering them in the inner template is not an alternative, since [RadioElement]
// allows Radio and Label at most one render each. Call RadioGroup from the template
// that renders the group to avoid the condition entirely.
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
