package jawstest

import (
	"errors"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/ui"
)

func TestTemplate_Missing(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	err := rq.Template("missingtemplate", nil, nil)
	if !errors.Is(err, ui.ErrMissingTemplate) {
		t.Error("wrong error", err)
	}
	if !strings.Contains(err.Error(), "missingtemplate") {
		t.Error("wrong error", err)
	}
}

func TestTemplate_String(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	dot := 123
	tmpl := ui.NewTemplate("testtemplate", dot)

	if tmpl.String() != `{"testtemplate", 123}` {
		t.Fatalf("unexpected template string: %q", tmpl.String())
	}
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	dot := &testUpdater{}
	tmpl := ui.NewTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.called != 1 {
		t.Error(dot.called)
	}
}

type testUpdater struct {
	called int
}

func (tu *testUpdater) JawsUpdate(*jaws.Element) {
	tu.called++
}
