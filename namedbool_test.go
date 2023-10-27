package jaws

import (
	"html/template"
	"testing"
)

func TestNamedBool(t *testing.T) {
	is := newTestHelper(t)

	nba := NewNamedBoolArray()
	nba.Add("1", "one")
	nb := nba.data[0]

	rq := newTestRequest()
	e := rq.NewElement(NewUiCheckbox(nb))
	defer rq.Close()

	is.Equal(nba, nb.Array())
	is.Equal("1", nb.Name())
	is.Equal(template.HTML("one"), nb.Html())

	is.Equal(nb.Name(), nb.JawsGetString(nil))
	is.Equal(nb.Html(), nb.JawsGetHtml(nil))

	is.NoErr(nb.JawsSetBool(e, true))
	is.True(nb.Checked())
	is.Equal(nb.Checked(), nb.JawsGetBool(nil))
}
