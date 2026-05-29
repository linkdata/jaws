package jawstree

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
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

func TestTree(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()

	mux := http.NewServeMux()
	err = jw.Setup(mux.Handle, "/", Setup)
	maybeError(t, err)

	go jw.Serve()
	rq := jaws.NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()

	root, err := os.OpenRoot(".")
	maybeError(t, err)
	defer root.Close()

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

	rootnode.JawsPathSet(elem, changed[0].ID+".selected", "false")
	select {
	case <-t.Context().Done():
	case msg := <-rq.OutCh:
		if s := "jawstreeSetPath={\"tree\":\"tree\",\"id\":\"children.1.children.1\",\"set\":false}"; msg.Data != s {
			t.Errorf("unexpected data: %q", msg.Data)
		}
	}
}
