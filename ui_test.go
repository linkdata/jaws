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
	id      string
	val     string
	gotCall chan struct{}
}

func (tu testUi) JawsUi(rq *Request, attrs ...string) template.HTML {
	return template.HTML(fmt.Sprintf(`<test jid="%s" %s>%s</test>`, rq.RegisterEventFn(tu.id, tu.JawsEvent), strings.Join(attrs, " "), tu.val))
}

func (tu testUi) JawsEvent(rq *Request, evt what.What, id, val string) (err error) {
	close(tu.gotCall)
	return
}

func Test_Ui(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	attrs := []string{"whut=0", "bah"}
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	tu := &testUi{id: elemId, val: elemVal, gotCall: gotCall}

	h := rq.Ui(tu)
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Ui(tu, attrs...)
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))
	is.True(strings.Contains(string(h), strings.Join(attrs, " ")))

	rq.inCh <- wsMsg{jid: jidForTag(rq.Request, elemId), What: what.Input, Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}
