package jawstree

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func runJawstreeJSSnippet(t *testing.T, snippet string) string {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node executable not available")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	jsPath := filepath.Join(filepath.Dir(file), "assets", "jawstree.js")

	script := `
const fs = require("fs");
const src = fs.readFileSync(process.argv[1], "utf8");
global.window = {};
eval(src);
` + snippet

	cmd := exec.Command(node, "-e", script, jsPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("node failed: %v\n%s", err, out.String())
	}
	return out.String()
}

// jsMock installs a faithful stand-in for the vendored Quercus Treeview, plus document
// and jaws stubs. selectNodeById replicates the real _selectNode (treeview.js:503-628)
// exactly, including its collateral effects the adapter must survive: a single-select
// deselect clears the WHOLE selection, and a cascade select/deselect toggles every
// descendant. It also fires onSelectionChange on every selectNodeById, like the real
// widget, so the adapter's echo guard is exercised. A mock that omitted these quirks is
// what let earlier reconcile bugs pass; keep it in lockstep with _selectNode.
const jsMock = `
var sends = [];
global.jaws = { readyState: 1, send: function (s) { sends.push(s); } };
var containers = {};
function addContainer(id) {
	var elem = { hidden: true };
	containers[id] = elem;
	return elem;
}
var container = addContainer("Jid.1");
addContainer("Jid.2");
addContainer("Jid.3");
global.document = { getElementById: function (id) { return containers[id] || null; } };

function Treeview(options) {
	this.options = options;
	this.treeviewContainer = document.getElementById(options.containerId);
	this.selected = new Set();
	this._nodes = {};
	var self = this;
	(function index(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) {
			self._nodes[n.id] = { selectable: n.selectable, childIds: (n.children || []).map(function (c) { return c.id; }) };
			index(n.children);
		});
	})(options.data);
	(function apply(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) { if (n.selected) { self.selectNodeById(n.id, true); } apply(n.children); });
	})(options.data);
}
Treeview.prototype._descendants = function (id) {
	var out = [];
	var self = this;
	(function rec(cid) {
		var node = self._nodes[cid];
		if (!node) { return; }
		node.childIds.forEach(function (k) { out.push(k); rec(k); });
	})(id);
	return out;
};
Treeview.prototype.getSelectedNodes = function () {
	return Array.from(this.selected).map(function (id) { return { id: id }; });
};
Treeview.prototype._fire = function () {
	if (this.options.onSelectionChange) { this.options.onSelectionChange(this.getSelectedNodes()); }
};
Treeview.prototype.selectNodeById = function (id, shouldSelect) {
	var node = this._nodes[id];
	if (!node) { return; }
	if (node.selectable === false && shouldSelect) { return; }
	var opt = this.options;
	var self = this;
	if (opt.cascadeSelectChildren) {
		var chain = [id].concat(this._descendants(id));
		if (shouldSelect) {
			if (!opt.multiSelectEnabled) { this.selected.clear(); }
			chain.forEach(function (k) { if (self._nodes[k].selectable !== false) { self.selected.add(k); } });
		} else {
			chain.forEach(function (k) { self.selected.delete(k); });
		}
	} else if (opt.multiSelectEnabled) {
		if (shouldSelect) { this.selected.add(id); } else { this.selected.delete(id); }
	} else {
		var wasSole = this.selected.has(id) && this.selected.size === 1;
		this.selected.clear();
		if (shouldSelect && !wasSole) { this.selected.add(id); }
	}
	this._fire();
};
Treeview.prototype.setData = function (data) {
	this.selected = new Set();
	this.options.data = data;
	this._nodes = {};
	var self = this;
	(function index(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) { self._nodes[n.id] = { selectable: n.selectable, childIds: (n.children || []).map(function (c) { return c.id; }) }; index(n.children); });
	})(data);
	(function apply(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) { if (n.selected) { self.selectNodeById(n.id, true); } apply(n.children); });
	})(data);
};
global.Treeview = Treeview;

function selectedIds(t) {
	return t.getSelectedNodes().map(function (n) { return n.id; }).sort();
}
`

func TestJawstreeJS_DecodeOptions(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
process.stdout.write(JSON.stringify({
	mixed: jawstreeDecodeOptions((1<<0)|(1<<2)|(1<<8)),
	disabled: jawstreeDecodeOptions(1<<6)
}));
`)
	var got struct {
		Mixed    map[string]bool `json:"mixed"`
		Disabled map[string]bool `json:"disabled"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !got.Mixed["searchEnabled"] || !got.Mixed["multiSelectEnabled"] || !got.Mixed["checkboxSelectionEnabled"] {
		t.Fatalf("mixed decode = %#v", got.Mixed)
	}
	if !got.Mixed["nodeSelectionEnabled"] {
		t.Fatalf("nodeSelectionEnabled should default true: %#v", got.Mixed)
	}
	if got.Disabled["nodeSelectionEnabled"] {
		t.Fatalf("NodeSelectionDisabled bit must clear nodeSelectionEnabled: %#v", got.Disabled)
	}
}

func TestJawstreeJS_BitmapRoundTrip(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
var b = jawstreeEncodeBitmap(new Set([1, 3]), 5);
var back = Array.from(jawstreeDecodeBitmap(b, 5)).sort(function (a, b) { return a - b; });
process.stdout.write(JSON.stringify({ b: b, back: back }));
`)
	var got struct {
		B    string `json:"b"`
		Back []int  `json:"back"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if got.B != "Cg==" {
		t.Fatalf("bitmap = %q, want Cg==", got.B)
	}
	if !reflect.DeepEqual(got.Back, []int{1, 3}) {
		t.Fatalf("decoded = %v, want [1 3]", got.Back)
	}
}

func TestJawstreeJS_InitBuildsFromData(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [
	{ id: "children.0", name: "a" },
	{ id: "children.1", name: "b", selected: true }
] };
jawstreeInit({ jid: "Jid.1", options: (1<<2), data: data });
var t = container.jawsTreeview;
process.stdout.write(JSON.stringify({
	visible: !container.hidden,
	isTreeview: t instanceof Treeview,
	ownedByElement: document.getElementById("Jid.1").jawsTreeview === t,
	instanceGlobals: Object.keys(window).filter(function (key) { return key.startsWith("jawstree_"); }).length,
	selected: selectedIds(t),
	lastServerSet: Array.from(t.lastServerSet),
	count: t.jawsNodeCount,
	sends: sends
}));
`)
	var got struct {
		Visible         bool     `json:"visible"`
		IsTreeview      bool     `json:"isTreeview"`
		OwnedByElement  bool     `json:"ownedByElement"`
		InstanceGlobals int      `json:"instanceGlobals"`
		Selected        []string `json:"selected"`
		LastServerSet   []int    `json:"lastServerSet"`
		Count           int      `json:"count"`
		Sends           []string `json:"sends"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !got.Visible || !got.IsTreeview || !got.OwnedByElement {
		t.Fatalf("init did not unhide/build/register: %+v", got)
	}
	if got.InstanceGlobals != 0 {
		t.Fatalf("init retained %d Treeview instance globals, want 0", got.InstanceGlobals)
	}
	if !reflect.DeepEqual(got.Selected, []string{"children.1"}) {
		t.Fatalf("initial selection = %v, want [children.1]", got.Selected)
	}
	if !reflect.DeepEqual(got.LastServerSet, []int{2}) {
		t.Fatalf("lastServerSet = %v, want [2]", got.LastServerSet)
	}
	if got.Count != 3 {
		t.Fatalf("node count = %d, want 3 (root + 2)", got.Count)
	}
	if len(got.Sends) != 0 {
		t.Fatalf("init leaked outbound frames: %v", got.Sends)
	}
}

func TestJawstreeJS_InitReconcilesCascadeSelection(t *testing.T) {
	tests := []struct {
		name    string
		options string
	}{
		{"cascade_only", `(1<<7)`},
		{"multi_cascade", `(1<<2)|(1<<7)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "P", selected: true, children: [
	{ id: "children.0.children.0", name: "A" },
	{ id: "children.0.children.1", name: "B", selected: true }
] }] };
jawstreeInit({ jid: "Jid.1", options: `+tt.options+`, data: data });
var t = container.jawsTreeview;
process.stdout.write(JSON.stringify({
	selected: selectedIds(t),
	baseline: Array.from(t.lastServerSet).sort(function (a, b) { return a - b; }),
	sends: sends
}));
`)
			var got struct {
				Selected []string `json:"selected"`
				Baseline []int    `json:"baseline"`
				Sends    []string `json:"sends"`
			}
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("unexpected JSON %q: %v", raw, err)
			}
			if !reflect.DeepEqual(got.Selected, []string{"children.0", "children.0.children.1"}) {
				t.Fatalf("initial selection = %v, want parent and B only", got.Selected)
			}
			if !reflect.DeepEqual(got.Baseline, []int{1, 3}) {
				t.Fatalf("initial baseline = %v, want [1 3]", got.Baseline)
			}
			if len(got.Sends) != 0 {
				t.Fatalf("initial reconcile produced outbound frames: %v", got.Sends)
			}
		})
	}
}

// TestJawstreeJS_SelectionReconcile drives the reconcile against the faithful mock in
// the two modes whose collateral effects broke the naive one-pass reconcile: a
// single-select switch must leave only the new node (not clear everything), and a
// cascade parent-deselect must keep the still-desired children. It also covers an
// exact pruned cascade-only update.
func TestJawstreeJS_SelectionReconcile(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var flat = { children: [{ id: "children.0", name: "A" }, { id: "children.1", name: "B" }] };
jawstreeInit({ jid: "Jid.1", options: 0, data: flat }); // single-select
var s = containers["Jid.1"].jawsTreeview;
jawstreeSelection({ jid: "Jid.1", s: [1] });
var afterA = selectedIds(s);
jawstreeSelection({ jid: "Jid.1", s: [2] }); // switch A -> B
var afterSwitch = selectedIds(s);
jawstreeSelection({ jid: "Jid.1", s: [] });
var afterClear = selectedIds(s);

var tree = { children: [{ id: "children.0", name: "P", children: [
	{ id: "children.0.children.0", name: "c1" },
	{ id: "children.0.children.1", name: "c2" }
] }] };
jawstreeInit({ jid: "Jid.2", options: (1<<2)|(1<<7), data: tree }); // multi + cascade
var c = containers["Jid.2"].jawsTreeview;
jawstreeSelection({ jid: "Jid.2", s: [1, 2, 3] });
var afterAll = selectedIds(c);
jawstreeSelection({ jid: "Jid.2", s: [2, 3] }); // parent deselected server-side
var afterParentDrop = selectedIds(c);

jawstreeInit({ jid: "Jid.3", options: (1<<7), data: tree }); // cascade only
var o = containers["Jid.3"].jawsTreeview;
jawstreeSelection({ jid: "Jid.3", s: [1, 2] });
var afterPruned = selectedIds(o);
var prunedBaseline = Array.from(o.lastServerSet).sort();

process.stdout.write(JSON.stringify({
	afterA: afterA, afterSwitch: afterSwitch, afterClear: afterClear,
	afterAll: afterAll, afterParentDrop: afterParentDrop,
	afterPruned: afterPruned, prunedBaseline: prunedBaseline, sends: sends
}));
`)
	var got struct {
		AfterA          []string `json:"afterA"`
		AfterSwitch     []string `json:"afterSwitch"`
		AfterClear      []string `json:"afterClear"`
		AfterAll        []string `json:"afterAll"`
		AfterParentDrop []string `json:"afterParentDrop"`
		AfterPruned     []string `json:"afterPruned"`
		PrunedBaseline  []int    `json:"prunedBaseline"`
		Sends           []string `json:"sends"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.AfterA, []string{"children.0"}) {
		t.Fatalf("select A = %v, want [children.0]", got.AfterA)
	}
	if !reflect.DeepEqual(got.AfterSwitch, []string{"children.1"}) {
		t.Fatalf("single-select switch A->B = %v, want [children.1] only (blocker 1)", got.AfterSwitch)
	}
	if len(got.AfterClear) != 0 {
		t.Fatalf("clear = %v, want empty", got.AfterClear)
	}
	if !reflect.DeepEqual(got.AfterAll, []string{"children.0", "children.0.children.0", "children.0.children.1"}) {
		t.Fatalf("cascade select-all = %v", got.AfterAll)
	}
	if !reflect.DeepEqual(got.AfterParentDrop, []string{"children.0.children.0", "children.0.children.1"}) {
		t.Fatalf("cascade parent-drop = %v, want the two children kept (blocker 2)", got.AfterParentDrop)
	}
	if !reflect.DeepEqual(got.AfterPruned, []string{"children.0", "children.0.children.0"}) ||
		!reflect.DeepEqual(got.PrunedBaseline, []int{1, 2}) {
		t.Fatalf("cascade-only pruned update = %v baseline %v, want parent+c1 and [1 2]", got.AfterPruned, got.PrunedBaseline)
	}
	if len(got.Sends) != 0 {
		t.Fatalf("reconcile produced %d outbound frames, want 0 (echo guard)", len(got.Sends))
	}
}

func TestJawstreeJS_UnreachableReconcileDoesNotAdvanceBaseline(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "A" }, { id: "children.1", name: "B" }] };
jawstreeInit({ jid: "Jid.1", options: (1<<7), data: data });
var t = container.jawsTreeview;
jawstreeSelection({ jid: "Jid.1", s: [1] });
var before = Array.from(t.lastServerSet);
jawstreeSelection({ jid: "Jid.1", s: [1, 2] });
process.stdout.write(JSON.stringify({
	before: before,
	selected: Array.from(jawstreeSelectedIndexSet(t)).sort(),
	baseline: Array.from(t.lastServerSet).sort(),
	sends: sends
}));
`)
	var got struct {
		Before   []int    `json:"before"`
		Selected []int    `json:"selected"`
		Baseline []int    `json:"baseline"`
		Sends    []string `json:"sends"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Before, []int{1}) {
		t.Fatalf("converged baseline = %v, want [1]", got.Before)
	}
	if reflect.DeepEqual(got.Selected, []int{1, 2}) {
		t.Fatalf("unreachable selection unexpectedly converged: %v", got.Selected)
	}
	if !reflect.DeepEqual(got.Baseline, []int{1}) {
		t.Fatalf("baseline after failed reconcile = %v, want prior [1]", got.Baseline)
	}
	if len(got.Sends) != 0 {
		t.Fatalf("reconcile produced outbound frames: %v", got.Sends)
	}
}

func TestJawstreeJS_OnSelectionChangeSendsDelta(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }, { id: "children.1", name: "b" }] };
jawstreeInit({ jid: "Jid.1", options: (1<<2), data: data });
var t = containers["Jid.1"].jawsTreeview;
// A user selecting children.0 (index 1): the widget mutates and fires onSelectionChange.
t.selectNodeById("children.0", true);
process.stdout.write(JSON.stringify({ sends: sends, baseline: Array.from(t.lastServerSet) }));
`)
	var got struct {
		Sends    []string `json:"sends"`
		Baseline []int    `json:"baseline"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if len(got.Sends) != 1 {
		t.Fatalf("outbound frames = %v, want exactly one", got.Sends)
	}
	want := "Input\tJid.1\t{\"d\":{\"add\":[1],\"remove\":[]}}\n"
	if got.Sends[0] != want {
		t.Fatalf("outbound frame = %q, want %q", got.Sends[0], want)
	}
	if !reflect.DeepEqual(got.Baseline, []int{1}) {
		t.Fatalf("baseline advanced to %v on successful send, want [1]", got.Baseline)
	}
}

func TestJawstreeJS_CascadeOnlySendsAbsoluteSelection(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "P", children: [
	{ id: "children.0.children.0", name: "q", selected: true },
	{ id: "children.0.children.1", name: "r" }
] }] };
jawstreeInit({ jid: "Jid.1", options: (1<<7), data: data });
var t = container.jawsTreeview;
// Selecting P overlaps the already-selected q subtree. The absolute payload must
// carry q as well as the nodes newly added by this gesture.
t.selectNodeById("children.0", true);
t.selectNodeById("children.0", false);
process.stdout.write(JSON.stringify({ sends: sends, baseline: Array.from(t.lastServerSet).sort() }));
`)
	var got struct {
		Sends    []string `json:"sends"`
		Baseline []int    `json:"baseline"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	wantSends := []string{
		"Input\tJid.1\t{\"s\":[1,2,3]}\n",
		"Input\tJid.1\t{\"d\":{\"add\":[],\"remove\":[1,2,3]}}\n",
	}
	if !reflect.DeepEqual(got.Sends, wantSends) {
		t.Fatalf("cascade-only frames = %v, want absolute select and delta clear", got.Sends)
	}
	if len(got.Baseline) != 0 {
		t.Fatalf("cascade-only baseline after clear = %v, want empty", got.Baseline)
	}
}

func TestJawstreeJS_CascadeOnlyAbsoluteFallsBackToBitmap(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
jawstreeDeltaThreshold = 1;
var data = { children: [{ id: "children.0", name: "a" }] };
jawstreeInit({ jid: "Jid.1", options: (1<<7), data: data });
container.jawsTreeview.selectNodeById("children.0", true);
process.stdout.write(JSON.stringify(sends));
`)
	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got, []string{"Input\tJid.1\t{\"b\":\"Ag==\"}\n"}) {
		t.Fatalf("cascade-only bitmap frames = %v, want index 1 bitmap", got)
	}
}

// TestJawstreeJS_DroppedSendDoesNotAdvanceBaseline verifies that when the socket is
// not open the outgoing delta is not lost: the baseline is not advanced, so the next
// gesture re-diffs from it and carries the earlier change.
func TestJawstreeJS_DroppedSendDoesNotAdvanceBaseline(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }, { id: "children.1", name: "b" }] };
jawstreeInit({ jid: "Jid.1", options: (1<<2), data: data });
var t = containers["Jid.1"].jawsTreeview;
global.jawsCanSend = function () { return false; }; // socket not open
t.selectNodeById("children.0", true); // dropped
var afterDrop = { sends: sends.length, baseline: Array.from(t.lastServerSet) };
global.jawsCanSend = function () { return true; }; // socket opens
t.selectNodeById("children.1", true); // now sends both selections
var afterOpen = { sends: sends.slice(), baseline: Array.from(t.lastServerSet).sort() };
process.stdout.write(JSON.stringify({ afterDrop: afterDrop, afterOpen: afterOpen }));
`)
	var got struct {
		AfterDrop struct {
			Sends    int   `json:"sends"`
			Baseline []int `json:"baseline"`
		} `json:"afterDrop"`
		AfterOpen struct {
			Sends    []string `json:"sends"`
			Baseline []int    `json:"baseline"`
		} `json:"afterOpen"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if got.AfterDrop.Sends != 0 || len(got.AfterDrop.Baseline) != 0 {
		t.Fatalf("dropped send: sends=%d baseline=%v, want 0 sends and empty baseline", got.AfterDrop.Sends, got.AfterDrop.Baseline)
	}
	// multi-select adds both children; the re-diff after the socket opens carries both.
	if len(got.AfterOpen.Sends) != 1 {
		t.Fatalf("after open: %d frames, want one carrying both changes", len(got.AfterOpen.Sends))
	}
	if got.AfterOpen.Sends[0] != "Input\tJid.1\t{\"d\":{\"add\":[1,2],\"remove\":[]}}\n" {
		t.Fatalf("after open frame = %q, want add [1,2]", got.AfterOpen.Sends[0])
	}
	if !reflect.DeepEqual(got.AfterOpen.Baseline, []int{1, 2}) {
		t.Fatalf("baseline after open = %v, want [1 2]", got.AfterOpen.Baseline)
	}
}

// TestJawstreeJS_ScopedByJid covers one Tree rendered by two elements on a page: each
// jid has its own widget, and a selection update addresses exactly one of them.
func TestJawstreeJS_ScopedByJid(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }] };
// Distinct jids — two renders of one shared Tree.
jawstreeInit({ jid: "Jid.1", options: (1<<2), data: data });
jawstreeInit({ jid: "Jid.2", options: (1<<2), data: data });
var one = containers["Jid.1"].jawsTreeview, two = containers["Jid.2"].jawsTreeview;
jawstreeSelection({ jid: "Jid.1", s: [1] });
var afterOne = { one: selectedIds(one), two: selectedIds(two) };
jawstreeSelection({ jid: "Jid.2", s: [1] });
var afterTwo = { one: selectedIds(one), two: selectedIds(two) };
process.stdout.write(JSON.stringify({
	distinct: one !== two,
	instanceGlobals: Object.keys(window).filter(function (key) { return key.startsWith("jawstree_"); }).length,
	afterOne: afterOne, afterTwo: afterTwo
}));
`)
	var got struct {
		Distinct        bool `json:"distinct"`
		InstanceGlobals int  `json:"instanceGlobals"`
		AfterOne        struct {
			One []string `json:"one"`
			Two []string `json:"two"`
		} `json:"afterOne"`
		AfterTwo struct {
			One []string `json:"one"`
			Two []string `json:"two"`
		} `json:"afterTwo"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !got.Distinct || got.InstanceGlobals != 0 {
		t.Fatalf("expected two distinct element-owned widgets and no instance globals: %+v", got)
	}
	// Updating Jid.1 must not touch Jid.2, and vice-versa.
	if !reflect.DeepEqual(got.AfterOne.One, []string{"children.0"}) || len(got.AfterOne.Two) != 0 {
		t.Fatalf("update to Jid.1 leaked: one=%v two=%v", got.AfterOne.One, got.AfterOne.Two)
	}
	if !reflect.DeepEqual(got.AfterTwo.Two, []string{"children.0"}) {
		t.Fatalf("update to Jid.2 did not apply: %v", got.AfterTwo.Two)
	}
}

// TestJawstreeJS_RemovalReleasesInstance covers a shared Tree removed and later
// reinserted through a live Container. A detached widget must not remain globally
// addressable, while the replacement widget still receives Jid-scoped updates.
func TestJawstreeJS_RemovalReleasesInstance(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }] };
jawstreeInit({ jid: "Jid.1", options: (1<<2), data: data });
var oldContainer = containers["Jid.1"];
var old = oldContainer.jawsTreeview;
delete containers["Jid.1"]; // JaWS Remove detached the old element.
jawstreeSelection({ jid: "Jid.1", s: [1] });

jawstreeInit({ jid: "Jid.2", options: (1<<2), data: data });
var replacement = containers["Jid.2"].jawsTreeview;
jawstreeSelection({ jid: "Jid.2", s: [1] });

process.stdout.write(JSON.stringify({
	oldDetached: document.getElementById("Jid.1") === null,
	oldOwned: oldContainer.jawsTreeview === old,
	distinct: old !== replacement,
	oldSelection: selectedIds(old),
	replacementSelection: selectedIds(replacement),
	instanceGlobals: Object.keys(window).filter(function (key) { return key.startsWith("jawstree_"); }).length
}));
`)
	var got struct {
		OldDetached          bool     `json:"oldDetached"`
		OldOwned             bool     `json:"oldOwned"`
		Distinct             bool     `json:"distinct"`
		OldSelection         []string `json:"oldSelection"`
		ReplacementSelection []string `json:"replacementSelection"`
		InstanceGlobals      int      `json:"instanceGlobals"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !got.OldDetached || !got.OldOwned || !got.Distinct {
		t.Fatalf("unexpected remove/reinsert ownership: %+v", got)
	}
	if len(got.OldSelection) != 0 {
		t.Fatalf("detached widget received a stale update: %v", got.OldSelection)
	}
	if !reflect.DeepEqual(got.ReplacementSelection, []string{"children.0"}) {
		t.Fatalf("replacement selection = %v, want [children.0]", got.ReplacementSelection)
	}
	if got.InstanceGlobals != 0 {
		t.Fatalf("remove/reinsert retained %d Treeview instance globals, want 0", got.InstanceGlobals)
	}
}
