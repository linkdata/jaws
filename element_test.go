package jaws

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

func TestElement_Tag(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	tss := &teststringsetter{}
	e := rq.NewElement(NewUiText(tss))
	is.True(!e.HasTag(Tag("zomg")))
	e.Tag(Tag("zomg"))
	is.True(e.HasTag(Tag("zomg")))
	is.True(strings.Contains(e.String(), "zomg"))
}

func TestElement_Queued(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	tss := &teststringsetter{}
	e := rq.NewElement(NewUiText(tss))
	e.SetAttr("hidden", "")
	e.RemoveAttr("hidden")
	e.SetClass("bah")
	e.RemoveClass("bah")
	e.SetValue("foo")
	e.SetInner("meh")
	e.Append("<div></div>")
	e.Remove("some-id")
	e.Order([]jid.Jid{1, 2})
	replaceHtml := template.HTML(fmt.Sprintf("<div id=\"%s\"></div>", e.Jid().String()))
	e.Replace(replaceHtml)
	e.Delete()

	is.Equal(e.wsQueue, []wsMsg{
		{
			Data: "hidden\n",
			Jid:  e.jid,
			What: what.SAttr,
		},
		{
			Data: "hidden",
			Jid:  e.jid,
			What: what.RAttr,
		},
		{
			Data: "bah",
			Jid:  e.jid,
			What: what.SClass,
		},
		{
			Data: "bah",
			Jid:  e.jid,
			What: what.RClass,
		},
		{
			Data: "foo",
			Jid:  e.jid,
			What: what.Value,
		},
		{
			Data: "meh",
			Jid:  e.jid,
			What: what.Inner,
		},
		{
			Data: "<div></div>",
			Jid:  e.jid,
			What: what.Append,
		},
		{
			Data: "some-id",
			Jid:  e.jid,
			What: what.Remove,
		},
		{
			Data: fmt.Sprintf("%s %s", Jid(1).String(), Jid(2).String()),
			Jid:  e.jid,
			What: what.Order,
		},
		{
			Data: string(replaceHtml),
			Jid:  e.jid,
			What: what.Replace,
		},
		{
			Jid:  e.jid,
			What: what.Delete,
		},
	})
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
