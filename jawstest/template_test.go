package jawstest

import (
	"errors"
	"strings"
	"testing"
)

func TestTemplate_Missing(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	err := rq.Template("missingtemplate", nil, nil)
	if !errors.Is(err, ErrMissingTemplate) {
		t.Error("wrong error", err)
	}
	if !strings.Contains(err.Error(), "missingtemplate") {
		t.Error("wrong error", err)
	}
}

func TestTemplate_String(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	dot := 123
	tmpl := NewTemplate("testtemplate", dot)

	is.Equal(tmpl.String(), `{"testtemplate", 123}`)
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	dot := &testUi{}
	tmpl := NewTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.updateCalled != 1 {
		t.Error(dot.updateCalled)
	}
}
