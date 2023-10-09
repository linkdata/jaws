package jaws

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/jid"
	"github.com/matryer/is"
)

func TestElement(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	tss := &teststringsetter{s: "foo"}
	e := rq.NewElement(NewUiText(tss))
	e.Tag(Tag("zomg"))
	is.True(e.HasTag(Tag("zomg")))
	is.True(strings.Contains(e.String(), "zomg"))
	e.SetAttr("hidden", "")
	e.RemoveAttr("hidden")
	e.SetClass("bah")
	e.RemoveClass("bah")
	e.SetValue("foo")
	e.SetInner("meh")
	h := template.HTML(fmt.Sprintf("<div id=\"%s\"></div>", e.Jid().String()))
	e.Replace(h)
	e.Append(h)
	e.Remove("some-id")
	e.Order([]jid.Jid{1, 2})
	e.Delete()
}

func TestElement_ReplacePanicsOnMissingId(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	defer func() {
		if x := recover(); x == nil {
			is.Fail()
		}
	}()
	tss := &teststringsetter{s: "foo"}
	e := rq.NewElement(NewUiText(tss))
	e.Replace(template.HTML("<div id=\"wrong\"></div>"))
	is.Fail()
}
