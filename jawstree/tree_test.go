package jawstree

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func maybeError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if x := recover(); x == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func mustNew(t *testing.T, l sync.Locker, root *Node, options ...Option) *Tree {
	t.Helper()
	tree, err := New(l, root, options...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tree
}

// readCall reads ch until it yields a what.Call whose data starts with prefix,
// failing after a deadline.
func readCall(t *testing.T, ch <-chan wire.WsMsg, prefix string) wire.WsMsg {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg := <-ch:
			if msg.What == what.Call && strings.HasPrefix(msg.Data, prefix) {
				return msg
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q message", prefix)
		}
	}
}

func TestNewSetsIDsIndexAndParents(t *testing.T) {
	root := &Node{Name: "root", Children: []*Node{
		{Name: "a", Children: []*Node{{Name: "one"}, {Name: "two"}}},
		{Name: "b"},
	}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	// Positional-path IDs and dense preorder index.
	a := root.Children[0]
	if a.ID != "children.0" || a.Children[1].ID != "children.0.children.1" || root.Children[1].ID != "children.1" {
		t.Fatalf("unexpected IDs: %q %q %q", a.ID, a.Children[1].ID, root.Children[1].ID)
	}
	wantOrder := []*Node{root, a, a.Children[0], a.Children[1], root.Children[1]}
	if !reflect.DeepEqual(tree.byIndex, wantOrder) {
		t.Fatalf("byIndex order = %v", tree.byIndex)
	}
	// Parent back-pointers.
	if a.Parent != root || a.Children[0].Parent != a || root.Parent != nil {
		t.Fatal("parent back-pointers not set")
	}
}

func TestNewStripsNilChildrenKeepingDenseIDs(t *testing.T) {
	a := &Node{Name: "a"}
	b := &Node{Name: "b"}
	root := &Node{Name: "root", Children: []*Node{nil, a, b}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	if len(root.Children) != 2 || root.Children[0] != a || root.Children[1] != b {
		t.Fatalf("New did not strip the nil child: %#v", root.Children)
	}
	if a.ID != "children.0" || b.ID != "children.1" {
		t.Fatalf("dense IDs expected, got a=%q b=%q", a.ID, b.ID)
	}
	// Index 1 is a, index 2 is b; a wire delta targeting index 2 must select b only.
	tree.Lock()
	changed, err := tree.applyClientDelta([]int{2}, nil)
	tree.Unlock()
	maybeError(t, err)
	if !changed || !b.Selected || a.Selected {
		t.Fatalf("index 2 should select b only: a=%v b=%v", a.Selected, b.Selected)
	}
}

func TestTreeSetSelectedNamePath(t *testing.T) {
	build := func() *Node {
		root := &Node{Name: "root"}
		root.Children = []*Node{{Name: "a"}, {Name: "b"}}
		return root
	}
	var mu deadlock.RWMutex

	// Single-select: one node is fine.
	single := mustNew(t, &mu, build())
	maybeError(t, single.SetSelected([][]string{{"b"}}))
	if got := single.GetSelected(); !reflect.DeepEqual(got, [][]string{{"b"}}) {
		t.Fatalf("single selected = %#v, want [[b]]", got)
	}
	// Single-select rejects selecting two nodes.
	if err := single.SetSelected([][]string{{"a"}, {"b"}}); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("single multi-select err = %v, want ErrInvalidSelection", err)
	}

	// Multi-select: both nodes selectable.
	var mu2 deadlock.RWMutex
	multi := mustNew(t, &mu2, build(), MultiSelectEnabled)
	maybeError(t, multi.SetSelected([][]string{{"a"}, {"b"}}))
	if got := multi.GetSelected(); !reflect.DeepEqual(got, [][]string{{"a"}, {"b"}}) {
		t.Fatalf("multi selected = %#v, want [[a] [b]]", got)
	}
}

func TestTreeSetSelectedDuplicateSiblingNames(t *testing.T) {
	build := func() *Node {
		root := &Node{Name: "root"}
		root.Children = []*Node{{Name: "dup"}, {Name: "dup"}, {Name: "uniq"}}
		return root
	}
	// Multi-select: a shared name-path selects both same-named siblings; GetSelected
	// reports one path per selected node (duplicated). This documents the lossiness.
	var mu deadlock.RWMutex
	multi := mustNew(t, &mu, build(), MultiSelectEnabled)
	maybeError(t, multi.SetSelected([][]string{{"dup"}}))
	if got := multi.GetSelected(); !reflect.DeepEqual(got, [][]string{{"dup"}, {"dup"}}) {
		t.Fatalf("selected = %#v, want two identical name-paths", got)
	}
	// Single-select: the shared name-path matches two nodes, which the policy rejects.
	var mu2 deadlock.RWMutex
	single := mustNew(t, &mu2, build())
	if err := single.SetSelected([][]string{{"dup"}}); !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("single dup err = %v, want ErrInvalidSelection", err)
	}
}

func TestTreeWalkAndGetNames(t *testing.T) {
	root := &Node{Name: "root", Children: []*Node{{Name: "a"}, {Name: "b"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	var walked []string
	tree.Walk(func(jsPath string, node *Node) {
		walked = append(walked, jsPath+":"+node.Name)
	})
	if !reflect.DeepEqual(walked, []string{":root", "children.0:a", "children.1:b"}) {
		t.Fatalf("walked = %#v", walked)
	}
	if got := root.Children[0].GetNames(); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("GetNames = %#v", got)
	}
}

func TestTreeRenderEmitsContainerAndInit(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()
	<-rq.ReadyCh

	root := &Node{Children: []*Node{{Name: "Documents"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root, InitiallyExpanded)
	elem := rq.NewElement(tree)

	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))
	rendered := sb.String()

	if !strings.Contains(rendered, `id="`+elem.Jid().String()+`"`) || !strings.Contains(rendered, " hidden></div>") {
		t.Fatalf("rendered view is missing its managed Jid container: %q", rendered)
	}
	// The JsVar shadow model is gone: no data-jawsname/data-jawsdata, no per-render script.
	if strings.Contains(rendered, "data-jawsname") || strings.Contains(rendered, "data-jawsdata") {
		t.Fatalf("rendered view still carries JsVar shadow wiring: %q", rendered)
	}
	if strings.Contains(rendered, "<script") {
		t.Fatalf("rendered view contains a per-render script: %q", rendered)
	}

	rq.InCh <- wire.WsMsg{}
	msg := readCall(t, rq.OutCh, "jawstreeInit=")
	if msg.Jid != elem.Jid() {
		t.Fatalf("init call Jid = %s, want %s", msg.Jid, elem.Jid())
	}
	tree.RLock()
	want := "jawstreeInit=" + tree.initPayloadLocked(elem.Jid().String())
	tree.RUnlock()
	if msg.Data != want {
		t.Fatalf("init data = %q, want %q", msg.Data, want)
	}
	if !strings.Contains(msg.Data, "Documents") {
		t.Fatalf("init data is missing the node tree: %q", msg.Data)
	}
}

func TestTreeRenderPreservesCallerParams(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	rq := jw.NewRequest(nil)

	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, &Node{})
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, []any{template.HTMLAttr(`class="tree"`)}))
	if !strings.Contains(sb.String(), `class="tree"`) {
		t.Fatalf("rendered view lost caller attributes: %q", sb.String())
	}
}

func TestTreeRenderDistinctIdentities(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	rq := jw.NewRequest(nil)

	var mu1, mu2 deadlock.RWMutex
	tree1 := mustNew(t, &mu1, &Node{Children: []*Node{{Name: "one"}}})
	tree2 := mustNew(t, &mu2, &Node{Children: []*Node{{Name: "two"}}})
	elem1 := rq.NewElement(tree1)
	elem2 := rq.NewElement(tree2)
	var b1, b2 strings.Builder
	maybeError(t, elem1.JawsRender(&b1, nil))
	maybeError(t, elem2.JawsRender(&b2, nil))

	if tree1.key == tree2.key {
		t.Fatalf("trees share generated key %q", tree1.key)
	}
	if elem1.Jid() == elem2.Jid() {
		t.Fatalf("trees share managed container Jid %s", elem1.Jid())
	}
}

// errWrite is the sentinel returned by failWriter.
var errWrite = errors.New("write failed")

// failWriter fails the failOn-th Write call (1-based) and succeeds on every other.
type failWriter struct {
	failOn int
	n      int
}

func (fw *failWriter) Write(p []byte) (int, error) {
	fw.n++
	if fw.n == fw.failOn {
		return 0, errWrite
	}
	return len(p), nil
}

func TestTreeRenderWriteError(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()

	root := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)
	elem := rq.NewElement(tree)
	if err := elem.JawsRender(&failWriter{failOn: 1}, nil); !errors.Is(err, errWrite) {
		t.Errorf("JawsRender err = %v, want %v", err, errWrite)
	}
}

func TestTreeJawsUpdateSendsSelection(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()
	<-rq.ReadyCh

	root := &Node{Name: "root", Children: []*Node{{Name: "a"}, {Name: "b"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))
	rq.InCh <- wire.WsMsg{}
	readCall(t, rq.OutCh, "jawstreeInit=") // drain the initializer

	maybeError(t, tree.SetSelected([][]string{{"b"}}))
	tree.JawsUpdate(elem)
	rq.InCh <- wire.WsMsg{}
	msg := readCall(t, rq.OutCh, "jawstreeSelection=")
	tree.RLock()
	want := "jawstreeSelection=" + tree.selectionPayloadLocked(elem.Jid().String())
	tree.RUnlock()
	if msg.Data != want {
		t.Fatalf("selection data = %q, want %q", msg.Data, want)
	}
	// b is preorder index 2; the payload must decode to it (sparse or bitmap form).
	got := decodeSelectionPayload(t, strings.TrimPrefix(msg.Data, "jawstreeSelection="), len(tree.byIndex))
	if !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("selection = %v, want [2] (payload %q)", got, msg.Data)
	}
}

// collectSelectionMessages drains ch, returning the jawstreeSelection calls seen
// before a short idle, and fails if none arrive before the deadline.
func collectSelectionMessages(t *testing.T, ch <-chan wire.WsMsg) (msgs []wire.WsMsg) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	idle := time.NewTimer(time.Hour)
	if !idle.Stop() {
		<-idle.C
	}
	defer idle.Stop()
	for {
		var idleC <-chan time.Time
		if len(msgs) > 0 {
			idleC = idle.C
		}
		select {
		case <-t.Context().Done():
			t.Fatal("test context expired while waiting for jawstreeSelection messages")
		case <-deadline.C:
			if len(msgs) == 0 {
				t.Fatal("timed out waiting for jawstreeSelection message")
			}
			return msgs
		case <-idleC:
			return msgs
		case msg := <-ch:
			if strings.Contains(msg.Data, "jawstreeSelection=") {
				msgs = append(msgs, msg)
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(250 * time.Millisecond)
			}
		}
	}
}

func TestTreeDirtySharedSendsOneSelectionPerRequest(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()

	rq1 := jawstest.NewTestRequest(jw, nil)
	defer rq1.Close()
	rq2 := jawstest.NewTestRequest(jw, nil)
	defer rq2.Close()
	<-rq1.ReadyCh
	<-rq2.ReadyCh

	root := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	elem1 := rq1.NewElement(tree)
	var sb1 strings.Builder
	maybeError(t, elem1.JawsRender(&sb1, nil))
	_ = rq2.NewElement(ui.NewDiv("placeholder")) // shift rq2's Jid so identities differ
	elem2 := rq2.NewElement(tree)
	var sb2 strings.Builder
	maybeError(t, elem2.JawsRender(&sb2, nil))

	for _, rq := range []*jawstest.TestRequest{rq1, rq2} {
		rq.InCh <- wire.WsMsg{}
		readCall(t, rq.OutCh, "jawstreeInit=")
	}

	maybeError(t, tree.SetSelected([][]string{{"child"}}))
	tree.Dirty(jw)

	msgs1 := collectSelectionMessages(t, rq1.OutCh)
	msgs2 := collectSelectionMessages(t, rq2.OutCh)
	if len(msgs1) != 1 || len(msgs2) != 1 {
		t.Fatalf("selection calls per request = %d,%d, want 1,1", len(msgs1), len(msgs2))
	}
	for _, msg := range append(msgs1, msgs2...) {
		if !strings.Contains(msg.Data, `"key":`+strconv.Quote(tree.key)) {
			t.Errorf("update missing tree key %q: %q", tree.key, msg.Data)
		}
	}
}

func TestTreeJawsInputSelectsAndReconverges(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()
	<-rq.ReadyCh

	root := &Node{Name: "root", Children: []*Node{{Name: "a"}, {Name: "b"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))
	rq.InCh <- wire.WsMsg{}
	readCall(t, rq.OutCh, "jawstreeInit=")

	// A delta selecting preorder index 2 (node b) is applied to the shared tree.
	maybeError(t, tree.JawsInput(elem, `{"d":{"add":[2],"remove":[]}}`))
	if !root.Children[1].Selected || root.Children[0].Selected {
		t.Fatalf("JawsInput did not select b only: a=%v b=%v", root.Children[0].Selected, root.Children[1].Selected)
	}
	// The change dirties the tree, so this view reconverges via jawstreeSelection.
	rq.InCh <- wire.WsMsg{}
	msg := readCall(t, rq.OutCh, "jawstreeSelection=")
	got := decodeSelectionPayload(t, strings.TrimPrefix(msg.Data, "jawstreeSelection="), len(tree.byIndex))
	if !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("reconverge selection = %v, want [2] (payload %q)", got, msg.Data)
	}
}

func TestTreeJawsInputRejectResyncsOriginOnly(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()

	rqOrigin := jawstest.NewTestRequest(jw, nil)
	defer rqOrigin.Close()
	rqPeer := jawstest.NewTestRequest(jw, nil)
	defer rqPeer.Close()
	<-rqOrigin.ReadyCh
	<-rqPeer.ReadyCh

	root := &Node{Name: "root", Children: []*Node{{Name: "a"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	elemOrigin := rqOrigin.NewElement(tree)
	var b1 strings.Builder
	maybeError(t, elemOrigin.JawsRender(&b1, nil))
	_ = rqPeer.NewElement(ui.NewDiv("placeholder"))
	elemPeer := rqPeer.NewElement(tree)
	var b2 strings.Builder
	maybeError(t, elemPeer.JawsRender(&b2, nil))
	for _, rq := range []*jawstest.TestRequest{rqOrigin, rqPeer} {
		rq.InCh <- wire.WsMsg{}
		readCall(t, rq.OutCh, "jawstreeInit=")
	}

	// An out-of-range index is rejected; nothing is mutated, so no fan-out happens,
	// but the origin still gets a resync push to correct its optimistic DOM.
	maybeError(t, tree.JawsInput(elemOrigin, `{"d":{"add":[99],"remove":[]}}`))
	if root.Children[0].Selected {
		t.Fatal("rejected input mutated the tree")
	}
	rqOrigin.InCh <- wire.WsMsg{}
	if msg := readCall(t, rqOrigin.OutCh, "jawstreeSelection="); !strings.Contains(msg.Data, `"s":[]`) {
		t.Fatalf("origin resync = %q, want empty selection", msg.Data)
	}
	// The peer must receive nothing.
	rqPeer.InCh <- wire.WsMsg{}
	select {
	case msg := <-rqPeer.OutCh:
		if strings.Contains(msg.Data, "jawstreeSelection=") {
			t.Fatalf("peer received an unexpected selection push: %q", msg.Data)
		}
	case <-time.After(300 * time.Millisecond):
		// expected: no selection message for the peer
	}
}

// TestTreeSingleSelectServerAuthoritative verifies that two clients selecting
// different nodes in a single-select tree converge to exactly one selection (the
// last committed), enforced server-side rather than by the clients.
func TestTreeSingleSelectServerAuthoritative(t *testing.T) {
	root := &Node{Name: "root", Children: []*Node{{Name: "a"}, {Name: "b"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root)

	// Client 1 selects a (index 1), client 2 then selects b (index 2). The second
	// select replaces the first: only b stays selected.
	tree.Lock()
	_, err := tree.applyClientDelta([]int{1}, nil)
	tree.Unlock()
	maybeError(t, err)
	tree.Lock()
	_, err = tree.applyClientDelta([]int{2}, nil)
	tree.Unlock()
	maybeError(t, err)

	if root.Children[0].Selected || !root.Children[1].Selected {
		t.Fatalf("single-select did not deselect the previous node: a=%v b=%v", root.Children[0].Selected, root.Children[1].Selected)
	}
	if got := tree.selectedIndexes(); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("selected indexes = %v, want [2]", got)
	}
}

// TestTreeConcurrentUpdateAndInput exercises JawsUpdate reading the shared tree
// under the read lock while JawsInput and SetSelected mutate it under the write lock.
// Run with -race.
func TestTreeConcurrentUpdateAndInput(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()
	<-rq.ReadyCh

	root := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, root, MultiSelectEnabled)
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-rq.OutCh:
			}
		}
	}()

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			payload := `{"d":{"add":[1],"remove":[]}}`
			if i%2 == 1 {
				payload = `{"d":{"add":[],"remove":[1]}}`
			}
			maybeError(t, tree.JawsInput(elem, payload))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tree.GetSelected()
			_ = tree.SetSelected([][]string{{"child"}})
			_ = tree.SetSelected(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tree.JawsUpdate(elem)
		}
	}()
	wg.Wait()
	close(stop)
}

func TestTreeEndToEndWithRoot(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	mux := http.NewServeMux()
	maybeError(t, jw.Setup(mux.Handle, "/", Setup))
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	defer rq.Close()
	<-rq.ReadyCh

	osRoot, err := os.OpenRoot(".")
	maybeError(t, err)
	defer func() { _ = osRoot.Close() }()
	rootnode, err := Root(osRoot, nil)
	maybeError(t, err)

	// Mark one node disabled before New so the initializer carries selectable:false.
	if len(rootnode.Children) == 0 {
		t.Fatal("Root produced no children")
	}
	rootnode.Children[0].Disabled = true

	var mu deadlock.RWMutex
	tree := mustNew(t, &mu, rootnode, SearchEnabled)
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))
	if strings.Contains(sb.String(), "<script") {
		t.Error("unexpected per-render script")
	}

	rq.InCh <- wire.WsMsg{}
	msg := readCall(t, rq.OutCh, "jawstreeInit=")

	// Every node's wire JSON appears in the init payload.
	numnodes := 0
	tree.Walk(func(jsPath string, node *Node) {
		b, err := json.Marshal(node)
		maybeError(t, err)
		if !strings.Contains(msg.Data, string(b)) {
			t.Errorf("init payload missing node %q", node.Name)
		}
		numnodes++
	})
	if numnodes == 0 {
		t.Fatal("no nodes rendered")
	}
	if !strings.Contains(msg.Data, `"selectable":false`) {
		t.Error("init payload missing selectable:false for the disabled node")
	}

	// There is no server-side init endpoint.
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jaws/.jawstree/tree/1", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("obsolete init endpoint status = %d, want 404", w.Code)
	}
}
