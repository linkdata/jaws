package ui

import (
	"context"
	"errors"
	"html/template"
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

// childJids returns the container's child Jids in order.
func childJids(t *testing.T, u *ContainerHelper) (jids []jaws.Jid) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, childElem := range u.contents {
		jids = append(jids, childElem.Jid())
	}
	return
}

// assertNoDOMMutation wakes the harness loop so any queued operations flush, then fails on
// an Append, Remove or Order — the traffic a container produces when it cannot reuse a
// child and has to replace the collection instead.
func assertNoDOMMutation(t *testing.T, tr *jawstest.TestRequest, round int) {
	t.Helper()
	tr.InCh <- wire.WsMsg{}
	for {
		select {
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Append, what.Remove, what.Order:
				t.Fatalf("round %d: unchanged collection queued %v for %v", round, msg.What, msg.Jid)
			}
		case <-time.After(300 * time.Millisecond):
			return
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

	before := childJids(t, &container.ContainerHelper)
	if len(before) != len(tc.rows) {
		t.Fatalf("child elements after render = %d, want %d", len(before), len(tc.rows))
	}

	for round := 1; round <= 3; round++ {
		container.JawsUpdate(elem)
		after := childJids(t, &container.ContainerHelper)
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

	before := childJids(t, &container.ContainerHelper)
	container.JawsUpdate(elem)
	after := childJids(t, &container.ContainerHelper)
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
	_ = jw.AddTemplateLookuper(template.Must(template.New("row").Parse(`<span>row</span>`)))

	// A map Dot satisfies the compile-time comparability of the any field but not the
	// runtime check.
	tc := &testContainer{contents: []jaws.UI{NewTemplate("div", "row", map[string]int{"a": 1})}}
	elem := rq.NewElement(NewContainer("div", tc))
	var sb strings.Builder
	_ = elem.JawsRender(&sb, nil)

	if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
		t.Fatalf("cancellation cause = %v, want wrapping %v", cause, tag.ErrNotUsableAsTag)
	}
}
