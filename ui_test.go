package jaws

import (
	"fmt"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

type testUi struct {
	tag     string
	jid     Jid
	val     string
	gotCall chan struct{}
}

func (tu *testUi) JawsUi(rq *Request, attrs ...string) template.HTML {
	tu.jid = rq.RegisterEventFn(Tag{tu.tag}, tu.JawsEvent)
	return template.HTML(fmt.Sprintf(`<test jid="%s" %s>%s</test>`, tu.jid, strings.Join(attrs, " "), tu.val))
}

func (tu *testUi) JawsEvent(rq *Request, evt what.What, id, val string) (err error) {
	close(tu.gotCall)
	return
}

func Test_Ui(t *testing.T) {
	const elemTag = "elem-tag"
	const elemVal = "elem-val"
	attrs := []string{"whut=0", "bah"}
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	tu := &testUi{tag: elemTag, val: elemVal, gotCall: gotCall}

	h := rq.Ui(tu)
	is.True(tu.jid != 0)

	is.True(strings.Contains(string(h), fmt.Sprintf(`jid="%s"`, tu.jid)))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Ui(tu, attrs...)
	is.True(strings.Contains(string(h), fmt.Sprintf(`jid="%s"`, tu.jid)))
	is.True(strings.Contains(string(h), elemVal))
	is.True(strings.Contains(string(h), strings.Join(attrs, " ")))

	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemTag), What: what.Input, Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}
