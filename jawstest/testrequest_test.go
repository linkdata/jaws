package jawstest

import (
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/ui"
)

type testRequestUpdater struct {
	called int
}

func (u *testRequestUpdater) JawsUpdate(*jaws.Element) {
	u.called++
}

func TestTestRequest_WrapperUIAndRegister(t *testing.T) {
	tj := newTestJaws()
	defer tj.Close()

	rq := tj.newRequest(nil)
	defer rq.Close()

	if err := rq.UI(ui.NewSpan(jaws.MakeHTMLGetter("ok"))); err != nil {
		t.Fatal(err)
	}
	if got := rq.BodyString(); !strings.Contains(got, `<span id="Jid.`) || !strings.Contains(got, `>ok</span>`) {
		t.Fatalf("unexpected body: %q", got)
	}

	up := &testRequestUpdater{}
	id := rq.Register(up)
	if !id.IsValid() {
		t.Fatalf("invalid jid: %v", id)
	}
	if up.called != 1 {
		t.Fatalf("expected updater called once, got %d", up.called)
	}
}
