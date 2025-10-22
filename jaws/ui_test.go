package jaws

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
)

func TestRequest_NewElement_DebugPanicsIfNotComparable(t *testing.T) {
	notHashableUI := struct {
		*UiSpan
		x map[int]int
	}{
		UiSpan: NewUiSpan(testHTMLGetter("foo")),
		x:      map[int]int{},
	}

	if newErrNotComparable(notHashableUI) == nil {
		t.FailNow()
	}

	if deadlock.Debug {
		defer func() {
			if x := recover(); x != nil {
				if err, ok := x.(error); ok {
					if !errors.Is(err, ErrNotComparable) {
						t.Errorf("%T", err)
					}
					return
				}
			}
			t.Fatal("expected ErrNotComparable")
		}()

		nextJid = 0
		rq := newTestRequest(t)
		defer rq.Close()

		rq.NewElement(notHashableUI)
		t.Fail()
	}
}

type testStringer struct{}

func (testStringer) String() string { return "foo" }

func TestRequest_JawsRender_DebugOutput(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	rq.Jaws.Debug = true
	rq.UI(&testUi{renderFn: func(e *Element, w io.Writer, params []any) error {
		e.Tag(Tag("footag"))
		e.Tag(e.Request)
		e.Tag(testStringer{})
		return nil
	}})
	h := rq.BodyString()
	t.Log(h)
	if !strings.Contains(h, "tags=[n/a]") {
		is.True(strings.Contains(h, "footag"))
		is.True(strings.Contains(h, "testUi"))
		is.True(strings.Contains(h, testStringer{}.String()))
	}
}

func TestRequest_InsideTemplate(t *testing.T) {
	var buf bytes.Buffer
	tr := newTestRequest(t)
	defer tr.Close()
	tp := newTestPage(tr)
	err := tp.render(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if x := buf.String(); x != testPageWant {
		t.Errorf("mismatch:\nwant %q\n got %q", testPageWant, x)
	}
}

func BenchmarkPageRender(b *testing.B) {
	tr := newTestRequest(nil)
	defer tr.Close()

	tp := newTestPage(tr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		tp.render(&buf)
	}
}

func BenchmarkPageUpdate(b *testing.B) {
	tr := newTestRequest(nil)
	defer tr.Close()
	tp := newTestPage(tr)
	var buf bytes.Buffer
	tp.render(&buf)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var x []byte
		tp.updateElems()
		for _, wsmsg := range tr.rq.wsQueue {
			x = wsmsg.Append(x)
		}
		tr.rq.wsQueue = tr.rq.wsQueue[:0]
	}
}
