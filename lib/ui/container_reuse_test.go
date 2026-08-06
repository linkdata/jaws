package ui

import (
	"context"
	"errors"
	"html/template"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// reuseRow is a comparable dot for the reuse fixtures.
type reuseRow struct{ id int }

// rebuildingContainer returns freshly built child values on every JawsContains call, the
// way an application container naturally does. Reuse therefore depends entirely on those
// values comparing equal.
type rebuildingContainer struct {
	rows  []*reuseRow
	build func(*reuseRow) jaws.UI
}

func (c *rebuildingContainer) JawsContains(elem *jaws.Element) (contents []jaws.UI) {
	for _, row := range c.rows {
		contents = append(contents, c.build(row))
	}
	return
}

// childJids returns the container Element's child Jids in order.
func childJids(t *testing.T, elem *jaws.Element) (jids []jaws.Jid) {
	t.Helper()
	for _, childElem := range containerElements(t, elem) {
		jids = append(jids, childElem.Jid())
	}
	return
}

// assertNoDOMMutation drains everything the container queued and fails on an Append, Remove
// or Order — the traffic it produces when it cannot reuse a child and has to replace the
// collection instead.
//
// The drain is bounded by a probe rather than by a timeout, so a slow machine cannot turn
// "the mutation has not arrived yet" into a pass. Two steps are needed, and for different
// reasons. The update ran on this goroutine, so its messages are already sitting in the
// queue; the unbuffered InCh send is consumed by the request loop, whose `continue` runs
// sendQueue before it can select anything else, forcing that batch out first. Only then is
// the Alert broadcast queued — otherwise getSendMsgs, which sorts by Jid, would place a
// Jid-0 Alert ahead of the element-addressed operations in a single flush and the barrier
// would prove nothing.
func assertNoDOMMutation(t *testing.T, tr *jawstest.TestRequest, round int) {
	t.Helper()
	tr.InCh <- wire.WsMsg{}

	probe := "drained " + strconv.Itoa(round)
	tr.BcastCh <- wire.Message{What: what.Alert, Data: probe}
	for {
		select {
		case msg, ok := <-tr.OutCh:
			if !ok {
				t.Fatalf("round %d: the request loop stopped before the probe arrived", round)
			}
			switch msg.What {
			case what.Append, what.Remove, what.Order:
				t.Fatalf("round %d: unchanged collection queued %v for %v", round, msg.What, msg.Jid)
			case what.Alert:
				if msg.Data == probe {
					return
				}
			}
		case <-tr.DoneCh:
			t.Fatalf("round %d: the request loop stopped before the probe arrived", round)
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: timed out waiting for the drain probe", round)
		}
	}
}

func newReuseRequest(t *testing.T) *jawstest.TestRequest {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	if err = jw.AddTemplateLookuper(template.Must(template.New("row").Parse(`<span>row</span>`))); err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh
	return tr
}

// TestContainer_RebuiltTemplateChildrenAreReused checks value-based reuse for a container
// that constructs equal Template children on every JawsContains call. Reuse means the child
// Jids stay unchanged and an unchanged collection queues no DOM mutation.
func TestContainer_RebuiltTemplateChildrenAreReused(t *testing.T) {
	tr := newReuseRequest(t)

	tc := &rebuildingContainer{
		rows: []*reuseRow{{id: 1}, {id: 2}, {id: 3}},
		build: func(row *reuseRow) jaws.UI {
			return NewTemplate("div", "row", row)
		},
	}
	container := NewContainer("div", tc)
	elem := tr.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	before := childJids(t, elem)
	if len(before) != len(tc.rows) {
		t.Fatalf("child elements after render = %d, want %d", len(before), len(tc.rows))
	}

	for round := 1; round <= 3; round++ {
		elem.JawsUpdate()
		after := childJids(t, elem)
		if len(after) != len(before) {
			t.Fatalf("round %d: child elements = %d, want %d", round, len(after), len(before))
		}
		for i := range after {
			if after[i] != before[i] {
				t.Fatalf("round %d: child %d Jid = %v, want %v (Element was not reused)",
					round, i, after[i], before[i])
			}
		}
		assertNoDOMMutation(t, tr, round)
	}
}

// TestContainer_StableChildrenAreReused checks identity-based reuse for children returned as
// the same values on every JawsContains call.
func TestContainer_StableChildrenAreReused(t *testing.T) {
	tr := newReuseRequest(t)

	tc := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("a")), NewSpan(testHTMLGetter("b"))}}
	container := NewContainer("div", tc)
	elem := tr.NewElement(container)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	before := childJids(t, elem)
	elem.JawsUpdate()
	after := childJids(t, elem)
	if len(after) != len(before) {
		t.Fatalf("child elements = %d, want %d", len(after), len(before))
	}
	for i := range after {
		if after[i] != before[i] {
			t.Fatalf("child %d Jid = %v, want %v", i, after[i], before[i])
		}
	}
	assertNoDOMMutation(t, tr, 1)
}

// TestContainer_NonComparableTemplateDotCancels checks the container key constraint for a
// Template value: a Dot that cannot be hashed terminates the Request before pool use.
func TestContainer_NonComparableTemplateDotCancels(t *testing.T) {
	jw, rq := newCoreRequest(t)
	if err := jw.AddTemplateLookuper(template.Must(template.New("row").Parse(`<span>row</span>`))); err != nil {
		t.Fatal(err)
	}

	// A map Dot satisfies the compile-time comparability of the any field but not the
	// runtime check.
	tc := &testContainer{contents: []jaws.UI{NewTemplate("div", "row", map[string]int{"a": 1})}}
	elem := rq.NewElement(NewContainer("div", tc))
	var sb strings.Builder
	// The render itself reports nothing: the container skips the children and still
	// closes its wrapper, so terminating the Request is the whole signal.
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("render = %v, want nil: the unusable child cancels the Request instead", err)
	}

	if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
		t.Fatalf("cancellation cause = %v, want wrapping %v", cause, tag.ErrNotUsableAsTag)
	}
}
