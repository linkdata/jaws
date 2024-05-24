package jaws

import (
	"testing"
)

func TestTemplate_String(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := NewTemplate("testtemplate", dot)

	is.Equal(tmpl.String(), `{"testtemplate", 123}`)
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	dot := &testUi{}
	tmpl := NewTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.updateCalled != 1 {
		t.Error(dot.updateCalled)
	}
}
