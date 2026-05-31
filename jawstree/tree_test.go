package jawstree

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/ui"
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

	root, err := os.OpenRoot(".")
	maybeError(t, err)
	defer func() { _ = root.Close() }()

	rootnode, err := Root(root, nil)
	maybeError(t, err)

	var rootmu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&rootmu, rootnode), SearchEnabled)
	elem := rq.NewElement(tree)

	var sb strings.Builder
	err = tree.JawsRender(elem, &sb, nil)
	maybeError(t, err)

	if strings.Contains(sb.String(), "DOMContentLoaded") {
		t.Error("unexpected inline script")
	}

	initURL := initScriptURL("tree", SearchEnabled)
	if !strings.Contains(sb.String(), `<script src="`+initURL+`"></script>`) {
		t.Errorf("missing init script URL: %q", initURL)
	}

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
	select {
	case <-t.Context().Done():
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
	case msg := <-rq.OutCh:
		if s := "jawstreeSetPath={\"tree\":\"tree\",\"id\":\"children.1.children.1\",\"set\":false}"; msg.Data != s {
			t.Errorf("unexpected data: %q", msg.Data)
		}
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

	rootnode := &Node{Name: "root", Children: []*Node{{Name: "child"}}}
	var mu deadlock.RWMutex
	tree := New("tree", ui.NewJsVar(&mu, rootnode))
	elem := rq.NewElement(tree)
	var sb strings.Builder
	maybeError(t, tree.JawsRender(elem, &sb, nil))

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

// TestTree_ConcurrentUpdateAndInput exercises JawsUpdate reading the shared node
// tree (via marshalJSON) under the JsVar read lock while another goroutine mutates
// a node under the write lock, exactly as JsVar.JawsInput does. Run with -race.
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

	// Drain broadcast traffic so JawsUpdate's JsCall never blocks or cancels.
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
		// Mutate node fields under the write lock, mirroring setPathLocked's jq.Set.
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
