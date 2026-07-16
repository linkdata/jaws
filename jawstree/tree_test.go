package jawstree

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
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

func TestNewPanicsOnNilJsVar(t *testing.T) {
	assertPanics(t, func() {
		New("tree", nil)
	})

	var mu deadlock.RWMutex
	assertPanics(t, func() {
		New("tree", ui.NewJsVar(&mu, (*Node)(nil)))
	})
}

func TestNewPanicsOnNegativeOptions(t *testing.T) {
	var mu deadlock.RWMutex
	assertPanics(t, func() {
		New("tree", ui.NewJsVar(&mu, &Node{}), Option(-1))
	})
}

func TestNode_SetSelectedMultiplePaths(t *testing.T) {
	root := &Node{}
	a := &Node{Name: "a", Parent: root}
	b := &Node{Name: "b", Parent: root}
	a1 := &Node{Name: "one", Parent: a}
	a2 := &Node{Name: "two", Parent: a}
	b1 := &Node{Name: "one", Parent: b}
	b2 := &Node{Name: "two", Parent: b}
	root.Children = []*Node{a, b}
	a.Children = []*Node{a1, a2}
	b.Children = []*Node{b1, b2}

	selected := [][]string{{"a", "one"}, {"b", "two"}}
	changed := root.SetSelected(selected)
	if !reflect.DeepEqual(changed, []*Node{a1, b2}) {
		t.Fatalf("changed = %#v, want %#v", changed, []*Node{a1, b2})
	}
	if got := root.GetSelected(); !reflect.DeepEqual(got, selected) {
		t.Fatalf("selected = %#v, want %#v", got, selected)
	}

	if changed = root.SetSelected(selected); len(changed) != 0 {
		t.Fatalf("changed after no-op = %#v, want none", changed)
	}

	selected = [][]string{{"a", "two"}}
	changed = root.SetSelected(selected)
	if !reflect.DeepEqual(changed, []*Node{a1, a2, b2}) {
		t.Fatalf("changed = %#v, want %#v", changed, []*Node{a1, a2, b2})
	}
	if got := root.GetSelected(); !reflect.DeepEqual(got, selected) {
		t.Fatalf("selected = %#v, want %#v", got, selected)
	}

	changed = root.SetSelected(nil)
	if !reflect.DeepEqual(changed, []*Node{a2}) {
		t.Fatalf("changed clearing selection = %#v, want %#v", changed, []*Node{a2})
	}
	if got := root.GetSelected(); len(got) != 0 {
		t.Fatalf("selected after clear = %#v, want none", got)
	}
}

func TestNode_SelectedDuplicateSiblingNamesCollapse(t *testing.T) {
	// Siblings sharing a name have identical name-paths, so selection by
	// name-path cannot tell them apart: selecting the shared path selects every
	// sibling, and GetSelected reports a name-path per selected node (duplicates).
	root := &Node{}
	dup1 := &Node{Name: "dup", Parent: root}
	dup2 := &Node{Name: "dup", Parent: root}
	uniq := &Node{Name: "uniq", Parent: root}
	root.Children = []*Node{dup1, dup2, uniq}

	changed := root.SetSelected([][]string{{"dup"}})
	if !reflect.DeepEqual(changed, []*Node{dup1, dup2}) {
		t.Fatalf("changed = %#v, want both same-named siblings %#v", changed, []*Node{dup1, dup2})
	}
	if got := root.GetSelected(); !reflect.DeepEqual(got, [][]string{{"dup"}, {"dup"}}) {
		t.Fatalf("selected = %#v, want two identical name-paths", got)
	}

	// Deselecting the shared path likewise clears every sibling that shares it.
	changed = root.SetSelected(nil)
	if !reflect.DeepEqual(changed, []*Node{dup1, dup2}) {
		t.Fatalf("changed clearing = %#v, want %#v", changed, []*Node{dup1, dup2})
	}
}

func TestTreeSelectionMethods(t *testing.T) {
	root := &Node{}
	a := &Node{Name: "a", Parent: root}
	b := &Node{Name: "b", Parent: root}
	root.Children = []*Node{a, b}

	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, root))

	changed := tree.SetSelected([][]string{{"b"}})
	if !reflect.DeepEqual(changed, []*Node{b}) {
		t.Fatalf("changed = %#v, want %#v", changed, []*Node{b})
	}
	if got := tree.GetSelected(); !reflect.DeepEqual(got, [][]string{{"b"}}) {
		t.Fatalf("selected = %#v, want b", got)
	}

	var walked []string
	tree.Walk(func(jsPath string, node *Node) {
		walked = append(walked, jsPath+":"+node.Name)
	})
	if !reflect.DeepEqual(walked, []string{":", "children.0:a", "children.1:b"}) {
		t.Fatalf("walked = %#v", walked)
	}
}

func TestTreeRenderEmitsRootDataAndQueuesInitializerForPageContainer(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	httpRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(httpRequest)

	var mu deadlock.RWMutex
	root := &Node{Children: []*Node{{Name: "Documents"}}}
	tree := New("mytree", ui.NewJsVar(&mu, root), InitiallyExpanded)
	elem := rq.NewElement(tree)

	var body bytes.Buffer
	if err := elem.JawsRender(&body, nil); err != nil {
		t.Fatal(err)
	}
	rendered := body.String()
	page := rendered + `<div id="mytree"></div>`

	if !strings.Contains(rendered, `data-jawsname="jawstreeroot_mytree"`) {
		t.Fatalf("rendered tree is missing root JsVar wiring: %q", rendered)
	}
	if !strings.Contains(rendered, `data-jawsdata=`) || !strings.Contains(rendered, "Documents") {
		t.Fatalf("rendered tree is missing serialized root data: %q", rendered)
	}
	if strings.Contains(rendered, "<script") {
		t.Fatalf("rendered tree contains a per-render script: %q", rendered)
	}
	if !strings.Contains(page, `<div id="mytree"></div>`) {
		t.Fatalf("page is missing the Quercus container: %q", page)
	}

	// Match the production lifecycle: initial rendering queues the Call before the
	// browser claims the Request and starts its WebSocket processing loop.
	if claimed := jw.UseRequest(rq.JawsKey, httpRequest); claimed != rq {
		t.Fatal("failed to claim rendered request")
	}
	go jw.Serve()
	inCh, outCh, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	select {
	case msg := <-outCh:
		if msg.What != what.Call || msg.Jid != elem.Jid() {
			t.Fatalf("initializer message = %+v, want element-scoped Call for %s", msg, elem.Jid())
		}
		if want := `jawsCallWhenReady={"id":` + strconv.Quote(elem.Jid().String()) + `,"path":"jawstreeInit","data":{"tree":"mytree","options":2}}`; msg.Data != want {
			t.Fatalf("initializer data = %q, want %q", msg.Data, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial tree initializer")
	}
}

func TestNewSetsParentPointers(t *testing.T) {
	// A hand-built tree has no Parent back-pointers; New must establish them,
	// since the name-path API matches nothing without them.
	root := &Node{Name: "root", Children: []*Node{
		{Name: "a", Children: []*Node{{Name: "one"}, nil, {Name: "two"}}},
		{Name: "b"},
	}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, root))

	aOne := root.Children[0].Children[0]
	if got := aOne.GetNames(); !reflect.DeepEqual(got, []string{"a", "one"}) {
		t.Fatalf("GetNames = %#v, want [a one]", got)
	}
	if !aOne.HasNames([]string{"a", "one"}) {
		t.Fatal("HasNames false for [a one]")
	}
	for _, tc := range []struct {
		name  string
		node  *Node
		names []string
		want  bool
	}{
		{"root empty matches", root, nil, true},
		{"root non-empty rejected", root, []string{"a"}, false},
		{"non-root empty rejected", aOne, nil, false},
		{"name mismatch rejected", aOne, []string{"a", "two"}, false},
		{"prefix mismatch rejected", aOne, []string{"b", "one"}, false},
	} {
		if got := tc.node.HasNames(tc.names); got != tc.want {
			t.Errorf("%s: HasNames(%#v) = %v, want %v", tc.name, tc.names, got, tc.want)
		}
	}
	changed := tree.SetSelected([][]string{{"a", "one"}})
	if !reflect.DeepEqual(changed, []*Node{aOne}) {
		t.Fatalf("changed = %#v, want %#v", changed, []*Node{aOne})
	}
	if got := tree.GetSelected(); !reflect.DeepEqual(got, [][]string{{"a", "one"}}) {
		t.Fatalf("selected = %#v, want [[a one]]", got)
	}
}

func TestNewStripsNilChildrenKeepingPathIndexConsistent(t *testing.T) {
	// A hand-built tree may contain a nil child. Before this was normalized, New
	// assigned IDs from the raw slice index while the wire array (marshalJSON)
	// compacted nils away, so the client's position-based path resolved to the
	// wrong node. After New, indices must be dense and a position-based path must
	// hit the matching node.
	a := &Node{Name: "a"}
	b := &Node{Name: "b"}
	root := &Node{Name: "root", Children: []*Node{nil, a, b}}
	var mu deadlock.RWMutex
	New("tree", ui.NewJsVar(&mu, root))

	if len(root.Children) != 2 || root.Children[0] != a || root.Children[1] != b {
		t.Fatalf("New did not strip the nil child: %#v", root.Children)
	}
	if a.ID != "children.0" || b.ID != "children.1" {
		t.Fatalf("dense IDs expected, got a=%q b=%q", a.ID, b.ID)
	}

	// The browser builds the path from the compacted wire-array position. Position
	// 1 is b; it must toggle b, not a, and must not be rejected.
	if err := root.JawsSetPath(nil, "children.1.selected", true); err != nil {
		t.Fatalf("JawsSetPath children.1: %v", err)
	}
	if !b.Selected || a.Selected {
		t.Fatalf("children.1 should select b only: a=%v b=%v", a.Selected, b.Selected)
	}
	// Position 0 is a; it must resolve rather than hit a (formerly nil) slot.
	if err := root.JawsSetPath(nil, "children.0.selected", true); err != nil {
		t.Fatalf("JawsSetPath children.0: %v", err)
	}
	if !a.Selected {
		t.Fatal("children.0 should select a")
	}
}

func TestTreeSelectionMethodsConcurrentWithInput(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()

	root := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, root))
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, tree.JawsRender(elem, &sb, nil))

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
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range iterations {
			value := "false"
			if i%2 == 0 {
				value = "true"
			}
			if err := tree.JawsInput(elem, "children.0.selected="+value); err != nil {
				t.Errorf("JawsInput: %v", err)
				return
			}
		}
	}()
	var totalChanged int
	go func() {
		defer wg.Done()
		for range iterations {
			_ = tree.GetSelected()
			totalChanged += len(tree.SetSelected([][]string{{"child"}}))
			tree.SetSelected(nil)
		}
	}()
	wg.Wait()
	close(stop)
	// The racing JawsInput goroutine has only iterations/2 "true" writes, so it
	// cannot pre-select the child ahead of every SetSelected call; if the write
	// half matches nothing this test passes vacuously, which this catches.
	if totalChanged == 0 {
		t.Error("SetSelected changed no nodes")
	}
}

func TestTree(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	mux := http.NewServeMux()
	err = jw.Setup(mux.Handle, "/", Setup)
	maybeError(t, err)

	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()
	<-rq.ReadyCh

	root, err := os.OpenRoot(".")
	maybeError(t, err)
	defer func() { _ = root.Close() }()

	rootnode, err := Root(root, nil)
	maybeError(t, err)

	var rootmu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&rootmu, rootnode), SearchEnabled)
	elem := rq.NewElement(tree)

	var sb strings.Builder
	err = elem.JawsRender(&sb, nil)
	maybeError(t, err)

	if strings.Contains(sb.String(), "<script") {
		t.Error("unexpected per-render script")
	}

	// The initial render queues one element-scoped initializer. Wake the request
	// loop and drain it before testing later tree updates.
	rq.InCh <- wire.WsMsg{}
	select {
	case msg := <-rq.OutCh:
		if msg.What != what.Call || msg.Jid != elem.Jid() || msg.Data != `jawsCallWhenReady={"id":`+strconv.Quote(elem.Jid().String())+`,"path":"jawstreeInit","data":{"tree":"tree","options":1}}` {
			t.Fatalf("unexpected initial tree message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial tree initializer")
	}

	// The generated route remains available to pages that request it directly,
	// although Tree rendering initializes through the preloaded adapter.
	initURL := initScriptURL("tree", SearchEnabled)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, initURL, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("init script status code = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != headerContentTypeJavaScript[0] {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != headerCacheControlNoStore[0] {
		t.Errorf("unexpected Cache-Control: %q", cc)
	}
	if got, want := w.Body.String(), string(appendInitScript(nil, "tree", SearchEnabled)); got != want {
		t.Errorf("unexpected init script:\n got %s\nwant %s\n", got, want)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/jaws/.jawstree/tree/not-a-number", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, badReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad init script status code = %d", w.Code)
	}

	badReq = httptest.NewRequest(http.MethodGet, "/jaws/.jawstree/tree-1/1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, badReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad init script tree status code = %d", w.Code)
	}

	// A valid tree name with a syntactically valid but negative options value is the
	// only input where the opt>=0 guard is the deciding rejection.
	badReq = httptest.NewRequest(http.MethodGet, "/jaws/.jawstree/tree/-1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, badReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("negative options status code = %d, want 400", w.Code)
	}

	numnodes := 0
	rendered := html.UnescapeString(sb.String())
	rootnode.Walk("", func(jsPath string, node *Node) {
		b, err := json.Marshal(node)
		maybeError(t, err)
		if !strings.Contains(rendered, string(b)) {
			t.Error(node.Name)
		}
		numnodes++
	})

	if numnodes == 0 {
		t.Log(sb.String())
		t.Fatal("no nodes rendered")
	}

	setnameslist := [][]string{{"assets", "jawstree.js"}}
	changed := rootnode.SetSelected(setnameslist)
	if len(changed) != 1 || changed[0].Name != "jawstree.js" {
		t.Fatal(changed)
	}

	getnameslist := rootnode.GetSelected()
	if !reflect.DeepEqual(setnameslist, getnameslist) {
		t.Log(setnameslist)
		t.Log(getnameslist)
		t.Fatal("selection mismatch")
	}

	changed[0].Disabled = true
	tree.JawsUpdate(elem)
	// This test drives a live request loop through the jawstest harness with the
	// Serve loop running in its own goroutine (go jw.Serve() above), so it cannot run
	// under testing/synctest, whose bubble requires every goroutine to be created and
	// durably block within it. The time.After guards are failure deadlines only, not
	// the happy path.
	select {
	case rq.InCh <- wire.WsMsg{}:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waking request loop")
	}
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for jawstreeSet message")
	case msg := <-rq.OutCh:
		if s := string(rootnode.marshalJSON(nil)); !strings.Contains(msg.Data, s) {
			t.Log(msg.Data)
			t.Error("msg data did not contain our JSON")
		}
		if !strings.Contains(msg.Data, `"selectable":false`) {
			t.Error("msg data did not contain selectable:false")
		}
	}

	// The value is a bool at runtime (JsVar unmarshals the wire value and the
	// JawsSetPath gate requires a bool), so pass one here rather than a string.
	rootnode.JawsPathSet(elem, changed[0].ID+".selected", false)
	select {
	case <-t.Context().Done():
		t.Fatal("expected a jawstreeSetPath broadcast")
	case msg := <-rq.OutCh:
		// The broadcast id is the node's own ID; derive the expectation from it
		// rather than a literal, which is fragile to the package directory listing.
		want := `jawstreeSetPath={"tree":"tree","id":` + strconv.Quote(changed[0].ID) + `,"set":false}`
		if msg.Data != want {
			t.Errorf("unexpected data: %q, want %q", msg.Data, want)
		}
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

// TestTree_JawsRenderWriteError verifies Tree.JawsRender propagates a write error
// from the hidden JsVar data element.
func TestTree_JawsRenderWriteError(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()

	rootnode := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, rootnode))

	elem := rq.NewElement(tree)
	if err := tree.JawsRender(elem, &failWriter{failOn: 1}, nil); !errors.Is(err, errWrite) {
		t.Errorf("JawsRender err = %v, want %v", err, errWrite)
	}
}

// TestTree_JawsPathSetIgnoresNonSelectedPath covers the early-return branch of
// JawsPathSet: only a ".selected" path broadcasts; any other path is ignored.
func TestTree_JawsPathSetIgnoresNonSelectedPath(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()
	<-rq.ReadyCh

	rootnode := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, rootnode))
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, elem.JawsRender(&sb, nil))

	// Flush the initializer queued by rendering so the assertion below isolates
	// the messages produced by JawsPathSet.
	rq.InCh <- wire.WsMsg{}
	select {
	case <-t.Context().Done():
		t.Fatal("expected a jawstreeInit message")
	case msg := <-rq.OutCh:
		if msg.What != what.Call || msg.Jid != elem.Jid() || msg.Data != `jawsCallWhenReady={"id":`+strconv.Quote(elem.Jid().String())+`,"path":"jawstreeInit","data":{"tree":"tree","options":0}}` {
			t.Fatalf("unexpected initial tree message: %+v", msg)
		}
	}

	child := rootnode.Children[0]
	// A non-".selected" path must be ignored (no broadcast)...
	child.JawsPathSet(elem, child.ID+".name", "renamed")
	// ...so the ".selected" broadcast is the first message on OutCh. The value is
	// a bool at runtime, so pass one rather than a string.
	child.JawsPathSet(elem, child.ID+".selected", true)

	select {
	case <-t.Context().Done():
		t.Fatal("expected a jawstreeSetPath message")
	case msg := <-rq.OutCh:
		if !strings.Contains(msg.Data, "jawstreeSetPath") || !strings.Contains(msg.Data, `"set":true`) {
			t.Fatalf("first message should be the .selected broadcast, got %q (a non-.selected path leaked a broadcast)", msg.Data)
		}
	}
}

func TestTree_DirtySharedTreeSendsOneUpdatePerRequest(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	go jw.Serve()
	rq1 := jawstest.NewTestRequest(jw, nil)
	if rq1 == nil {
		t.Fatal("nil test request 1")
	}
	defer rq1.Close()
	rq2 := jawstest.NewTestRequest(jw, nil)
	if rq2 == nil {
		t.Fatal("nil test request 2")
	}
	defer rq2.Close()
	<-rq1.ReadyCh
	<-rq2.ReadyCh

	rootnode := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, rootnode))

	elem1 := rq1.NewElement(tree)
	var sb1 strings.Builder
	maybeError(t, elem1.JawsRender(&sb1, nil))

	elem2 := rq2.NewElement(tree)
	var sb2 strings.Builder
	maybeError(t, elem2.JawsRender(&sb2, nil))

	tree.Lock()
	rootnode.Children[0].Disabled = true
	tree.Unlock()
	jw.Dirty(rootnode)

	msgs1 := collectJawstreeSetMessages(t, rq1.OutCh)
	msgs2 := collectJawstreeSetMessages(t, rq2.OutCh)
	if len(msgs1) != 1 {
		t.Fatalf("request 1 got %d jawstreeSet calls, want 1: %#v", len(msgs1), msgs1)
	}
	if len(msgs2) != 1 {
		t.Fatalf("request 2 got %d jawstreeSet calls, want 1: %#v", len(msgs2), msgs2)
	}
}

func collectJawstreeSetMessages(t *testing.T, ch <-chan wire.WsMsg) (msgs []wire.WsMsg) {
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
			t.Fatal("test context expired while waiting for jawstreeSet messages")
		case <-deadline.C:
			if len(msgs) == 0 {
				t.Fatal("timed out waiting for jawstreeSet message")
			}
			return msgs
		case <-idleC:
			return msgs
		case msg := <-ch:
			if strings.Contains(msg.Data, "jawstreeSet=") {
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

// TestTree_ConcurrentUpdateAndInput exercises JawsUpdate reading the shared node
// tree (via marshalJSON) under the JsVar read lock while another goroutine mutates a
// node under the write lock — the same RWMutex discipline JsVar uses when dispatching
// an inbound JawsInput. Run with -race.
func TestTree_ConcurrentUpdateAndInput(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()

	root, err := os.OpenRoot(".")
	maybeError(t, err)
	defer func() { _ = root.Close() }()

	rootnode, err := Root(root, nil)
	maybeError(t, err)

	var rootmu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&rootmu, rootnode))
	elem := rq.NewElement(tree)

	var sb strings.Builder
	maybeError(t, tree.JawsRender(elem, &sb, nil))

	// Drain any update traffic the request loop flushes while this race test runs.
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
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range iterations {
			tree.JawsUpdate(elem)
		}
	}()
	go func() {
		defer wg.Done()
		// Mutate node fields under the write lock JsVar holds while dispatching to
		// Node.JawsSetPath (the real path only writes Selected; this toggles more to
		// stress the lock, not to mirror the exact field set).
		for range iterations {
			tree.Lock()
			rootnode.Selected = !rootnode.Selected
			rootnode.Disabled = !rootnode.Disabled
			tree.Unlock()
		}
	}()
	wg.Wait()
	close(stop)
}
