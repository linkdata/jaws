package jaws

import (
	"io"
	"strings"
	"testing"
)

type testStringer struct{}

func (s testStringer) String() string {
	return "I_Am_A_testStringer"
}

func TestRequest_JawsRender_DebugOutput(t *testing.T) {

	is := testHelper{t}
	rq := newTestRequest()
	defer rq.Close()
	rq.Jaws.Debug = true
	h := string(rq.UI(&testUi{renderFn: func(e *Element, w io.Writer, params []any) {
		e.Tag(Tag("footag"))
		e.Tag(e.Request)
		e.Tag(testStringer{})
	}}))
	t.Log(h)
	is.True(strings.Contains(h, "footag"))
	is.True(strings.Contains(h, "*jaws.testUi"))
	is.True(strings.Contains(h, testStringer{}.String()))
}
