package jaws

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
)

type testStringer struct{}

func (s testStringer) String() string {
	return "I_Am_A_testStringer"
}

func TestRequest_NewElement_DebugPanicsIfNotComparable(t *testing.T) {
	notHashableUI := struct {
		*UiSpan
		x map[int]int
	}{
		UiSpan: NewUiSpan(testHtmlGetter("foo")),
		x:      map[int]int{},
	}

	if newErrNotComparable(notHashableUI) == nil {
		t.FailNow()
	}

	if !deadlock.Debug {
		return
	}

	defer func() {
		if x := recover(); x != nil {
			if err, ok := x.(error); ok {
				if !errors.Is(err, ErrNotComparable) {
					t.Errorf("%T", err)
				}
				return
			}
		}
		t.Fail()
	}()

	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	rq.NewElement(notHashableUI)
	t.Fail()
}

func TestRequest_JawsRender_DebugOutput(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
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
		is.True(strings.Contains(h, "*jaws.testUi"))
		is.True(strings.Contains(h, testStringer{}.String()))
	}
}

func TestRequest_InsideTemplate(t *testing.T) {
	jw := New()
	defer jw.Close()

	var buf bytes.Buffer
	tp := newTestPage(jw)
	err := tp.render(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if x := buf.String(); x != testPageWant {
		t.Errorf("mismatch:\nwant %q\n got %q", testPageWant, x)
	}
}

func BenchmarkRequest_InsideTemplate(b *testing.B) {
	jw := New()
	defer jw.Close()

	tp := newTestPage(jw)
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		tp.render(&buf)
	}
}
