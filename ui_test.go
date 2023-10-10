package jaws

import (
	"io"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestRequest_JawsRender_DebugOutput(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	rq.Jaws.Debug = true
	h := string(rq.UI(&testUi{renderFn: func(e *Element, w io.Writer, params []any) {
		e.Tag(Tag("footag"))
	}}))
	t.Log(h)
	is.True(strings.Contains(h, "footag"))
	is.True(strings.Contains(h, "*jaws.testUi"))
}
