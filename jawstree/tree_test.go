package jawstree

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

func maybeError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
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

	root, err := os.OpenRoot(".")
	maybeError(t, err)
	defer root.Close()

	rootnode, err := Root(root, nil)
	maybeError(t, err)

	var rootmu deadlock.RWMutex
	tree := New("tree", jaws.NewJsVar(&rootmu, rootnode), SearchEnabled)
	elem := rq.NewElement(tree)

	var sb strings.Builder
	err = tree.JawsRender(elem, &sb, nil)
	maybeError(t, err)

	if !strings.Contains(sb.String(), "DOMContentLoaded") {
		t.Error("missing DOMContentLoaded")
	}

	numnodes := 0
	rootnode.Walk("", func(jspath string, node *Node) {
		b, err := json.Marshal(node)
		maybeError(t, err)
		if !strings.Contains(sb.String(), string(b)) {
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
