package jawstree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	jawsassets "github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type dynamicTreeContainer struct {
	contents []jaws.UI
}

func (c *dynamicTreeContainer) JawsContains(*jaws.Element) []jaws.UI {
	return c.contents
}

func TestTreeDynamicLifecycleWithClientAssets(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()
	<-tr.ReadyCh

	// Render an initially empty live Container. The self-contained Tree enters the
	// page only through the subsequent Container update.
	var initial strings.Builder
	contents := &dynamicTreeContainer{}
	container := ui.NewContainer("div", contents)
	containerElem := tr.NewElement(container)
	if err := containerElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}

	root := &Node{Children: []*Node{{Name: "Documents"}}}
	var mu deadlock.RWMutex
	tree, err := New(&mu, root, InitiallyExpanded)
	if err != nil {
		t.Fatal(err)
	}
	contents.contents = []jaws.UI{tree}
	tr.Dirty(contents)

	frames := make([]wire.WsMsg, 0, 8)
	for len(frames) < 3 {
		select {
		case msg := <-tr.OutCh:
			frames = append(frames, msg)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for dynamic Tree messages; got %+v", frames)
		}
	}
	if frames[0].What != what.Append || frames[0].Jid != containerElem.Jid() {
		t.Fatalf("first dynamic message = %+v, want parent Append", frames[0])
	}
	if frames[1].What != what.Order || frames[1].Jid != containerElem.Jid() {
		t.Fatalf("second dynamic message = %+v, want parent Order", frames[1])
	}
	tree.RLock()
	wantInit := "jawstreeInit=" + tree.initPayloadLocked(frames[2].Jid.String())
	tree.RUnlock()
	if frames[2].What != what.Call || frames[2].Jid <= containerElem.Jid() || frames[2].Data != wantInit {
		t.Fatalf("third dynamic message = %+v, want child initializer Call", frames[2])
	}
	if strings.Contains(frames[0].Data, "<script") ||
		!strings.Contains(frames[0].Data, `id="`+frames[2].Jid.String()+`"`) {
		t.Fatalf("dynamic Tree Append has unexpected HTML: %q", frames[0].Data)
	}

	// Select a node server-side and dirty the tree immediately after insertion. The
	// production writer may coalesce this jawstreeSelection with the
	// Append/Order/initializer messages above. Only selection is synced (not
	// names/structure), so the update carries the selected index rather than a
	// relabel.
	if err := tree.SetSelected([][]string{{"Documents"}}); err != nil {
		t.Fatal(err)
	}
	tree.Dirty(jw)
	select {
	case msg := <-tr.OutCh:
		frames = append(frames, msg)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dynamic Tree update; got %+v", frames)
	}
	if frames[3].What != what.Call || frames[3].Jid != frames[2].Jid ||
		!strings.Contains(frames[3].Data, "jawstreeSelection=") {
		t.Fatalf("fourth dynamic message = %+v, want child jawstreeSelection Call", frames[3])
	}
	if got := decodeSelectionPayload(t, strings.TrimPrefix(frames[3].Data, "jawstreeSelection="), len(tree.byIndex)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("selection frame decoded to %v, want [1]", got)
	}

	// Remove the Tree through the same live Container, then reinsert the same shared
	// Tree. Reconciliation must assign a fresh child Jid, and the browser adapter must
	// not retain the detached Treeview after initializing the replacement.
	oldTreeJid := frames[2].Jid
	contents.contents = nil
	tr.Dirty(contents)
	select {
	case msg := <-tr.OutCh:
		frames = append(frames, msg)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dynamic Tree removal; got %+v", frames)
	}
	if frames[4].What != what.Remove || frames[4].Jid != containerElem.Jid() || frames[4].Data != oldTreeJid.String() {
		t.Fatalf("fifth dynamic message = %+v, want parent Remove of %s", frames[4], oldTreeJid)
	}

	contents.contents = []jaws.UI{tree}
	tr.Dirty(contents)
	for len(frames) < 8 {
		select {
		case msg := <-tr.OutCh:
			frames = append(frames, msg)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for dynamic Tree reinsertion; got %+v", frames)
		}
	}
	if frames[5].What != what.Append || frames[5].Jid != containerElem.Jid() {
		t.Fatalf("sixth dynamic message = %+v, want parent Append", frames[5])
	}
	if frames[6].What != what.Order || frames[6].Jid != containerElem.Jid() {
		t.Fatalf("seventh dynamic message = %+v, want parent Order", frames[6])
	}
	newTreeJid := frames[7].Jid
	if newTreeJid == oldTreeJid {
		t.Fatalf("reinserted Tree reused removed Jid %s", newTreeJid)
	}
	tree.RLock()
	wantReinit := "jawstreeInit=" + tree.initPayloadLocked(newTreeJid.String())
	tree.RUnlock()
	if frames[7].What != what.Call || newTreeJid <= containerElem.Jid() || frames[7].Data != wantReinit {
		t.Fatalf("eighth dynamic message = %+v, want replacement child initializer Call", frames[7])
	}
	if strings.Contains(frames[5].Data, "<script") ||
		!strings.Contains(frames[5].Data, `id="`+newTreeJid.String()+`"`) {
		t.Fatalf("reinserted Tree Append has unexpected HTML: %q", frames[5].Data)
	}

	var dynamicFrame []byte
	for i := range frames {
		dynamicFrame = frames[i].Append(dynamicFrame)
	}
	dynamicFrameJSON, err := json.Marshal(string(dynamicFrame))
	if err != nil {
		t.Fatal(err)
	}
	oldTreeJidJSON, err := json.Marshal(oldTreeJid.String())
	if err != nil {
		t.Fatal(err)
	}
	newTreeJidJSON, err := json.Marshal(newTreeJid.String())
	if err != nil {
		t.Fatal(err)
	}
	treeviewJS, err := assetsFS.ReadFile("assets/treeview.js")
	if err != nil {
		t.Fatal(err)
	}
	jawstreeJS, err := assetsFS.ReadFile("assets/jawstree.js")
	if err != nil {
		t.Fatal(err)
	}

	// Recreate the generated defer lifecycle with actual external scripts. The fake
	// WebSocket records whether core JaWS waits for the later deferred dependencies
	// before connecting, then delivers the genuine server frame.
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"jaws.js":     []byte(jawsassets.JavascriptText),
		"jawstree.js": jawstreeJS,
		"treeview.js": treeviewJS,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	htmlText := `<!doctype html><html><head>
<meta name="jawsKey" content="1">
<style>#result{position:fixed;inset:0;z-index:9999;background:rgb(255,0,0)}</style>
<script>
window.dynamicFrame = ` + string(dynamicFrameJSON) + `;
window.oldTreeJid = ` + string(oldTreeJidJSON) + `;
window.newTreeJid = ` + string(newTreeJidJSON) + `;
window.WebSocket = class {
	constructor() {
		this.readyState = 1;
		window.connectedAfterAssets =
			typeof window.jawstreeInit === "function" &&
			typeof window.Treeview === "function";
	}
	addEventListener(name, fn) {
		if (name === "message") {
			queueMicrotask(function() {
				fn({data: window.dynamicFrame});
			});
		}
	}
	send() {}
};
</script>
<script defer src="jaws.js"></script>
<script defer src="jawstree.js"></script>
<script defer src="treeview.js"></script>
</head><body>` + initial.String() + `<div id="result"></div>
<script>
window.addEventListener("DOMContentLoaded", function() {
	setTimeout(function() {
		const oldTarget = document.getElementById(window.oldTreeJid);
		const newTarget = document.getElementById(window.newTreeJid);
		const text = newTarget && newTarget.querySelector(".treeview-node-text");
		const selected = newTarget && newTarget.jawsTreeview && newTarget.jawsTreeview.getSelectedNodes();
		const treeviewGlobals = Object.keys(window).filter(function(key) {
			return key.startsWith("jawstree_");
		});
		if (window.connectedAfterAssets &&
			oldTarget === null &&
			newTarget && newTarget.jawsTreeview instanceof window.Treeview &&
			selected && selected.length === 1 && selected[0].id === "children.0" &&
			text && text.textContent === "Documents" && newTarget.querySelector("li.selected") &&
			newTarget.querySelector("ul") && !newTarget.hidden && treeviewGlobals.length === 0) {
			document.getElementById("result").style.background = "rgb(0,255,0)";
		}
	}, 0);
});
</script>
</body></html>`

	htmlPath := filepath.Join(dir, "dynamic-tree.html")
	if err := os.WriteFile(htmlPath, []byte(htmlText), 0o600); err != nil {
		t.Fatal(err)
	}
	jawstreeRunJsdomPage(t, htmlPath)
}
