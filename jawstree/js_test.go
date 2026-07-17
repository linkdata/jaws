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

// jsMock installs a stand-in Quercus Treeview plus document and jaws stubs. The mock
// Treeview mirrors the two behaviors the adapter must cope with: selectNodeById fires
// onSelectionChange (so the echo guard is exercised), and single-select selection
// replaces the previous selection.
const jsMock = `
var sends = [];
global.jaws = { readyState: 1, send: function (s) { sends.push(s); } };
var container = { hidden: true };
global.document = { getElementById: function (id) { return id === "Jid.1" ? container : null; } };

function Treeview(options) {
	this.options = options;
	this.selected = [];
	var self = this;
	this.treeviewContainer = {
		querySelector: function (sel) {
			var m = sel.match(/\[data-id="(.*)"\]/);
			var id = m ? m[1] : null;
			return { classList: { contains: function (c) { return c === 'selected' && self.selected.indexOf(id) !== -1; } } };
		}
	};
	(function walk(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) { if (n.selected) { self.selectNodeById(n.id, true); } walk(n.children); });
	})(options.data);
}
Treeview.prototype.getSelectedNodes = function () {
	return this.selected.map(function (id) { return { id: id }; });
};
Treeview.prototype._fire = function () {
	if (this.options.onSelectionChange) { this.options.onSelectionChange(this.getSelectedNodes()); }
};
Treeview.prototype.selectNodeById = function (id, set) {
	if (set) {
		if (!this.options.multiSelectEnabled && !this.options.cascadeSelectChildren) {
			this.selected = [id];
		} else if (this.selected.indexOf(id) === -1) {
			this.selected.push(id);
		}
	} else {
		this.selected = this.selected.filter(function (x) { return x !== id; });
	}
	this._fire();
};
Treeview.prototype.setData = function (data) {
	this.selected = [];
	this.options.data = data;
	var self = this;
	(function walk(nodes) {
		if (!nodes) { return; }
		nodes.forEach(function (n) { if (n.selected) { self.selectNodeById(n.id, true); } walk(n.children); });
	})(data);
};
global.Treeview = Treeview;
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
jawstreeInit({ key: "k", jid: "Jid.1", options: (1<<2), data: data });
var t = window["jawstree_k"];
process.stdout.write(JSON.stringify({
	visible: !container.hidden,
	isTreeview: t instanceof Treeview,
	selected: t.getSelectedNodes().map(function (n) { return n.id; }),
	lastServerSet: Array.from(t.lastServerSet),
	count: t.jawsNodeCount,
	sends: sends
}));
`)
	var got struct {
		Visible       bool     `json:"visible"`
		IsTreeview    bool     `json:"isTreeview"`
		Selected      []string `json:"selected"`
		LastServerSet []int    `json:"lastServerSet"`
		Count         int      `json:"count"`
		Sends         []string `json:"sends"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !got.Visible || !got.IsTreeview {
		t.Fatalf("init did not unhide/build: %+v", got)
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

func TestJawstreeJS_SelectionReconcile(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }, { id: "children.1", name: "b" }] };

// Multi-select: absolute set [1] then [2] reconciles by selectNodeById only.
jawstreeInit({ key: "multi", jid: "Jid.1", options: (1<<2), data: data });
var m = window["jawstree_multi"];
jawstreeSelection({ key: "multi", s: [1] });
var afterOne = m.getSelectedNodes().map(function (n) { return n.id; });
jawstreeSelection({ key: "multi", s: [1, 2] });
var afterBoth = m.getSelectedNodes().map(function (n) { return n.id; }).sort();
var sendsAfterReconcile = sends.length;

// Single-select: [1] then [2] must replace, leaving only children.1.
container.hidden = true;
jawstreeInit({ key: "single", jid: "Jid.1", options: 0, data: data });
var s = window["jawstree_single"];
jawstreeSelection({ key: "single", s: [1] });
jawstreeSelection({ key: "single", s: [2] });
var single = s.getSelectedNodes().map(function (n) { return n.id; });

process.stdout.write(JSON.stringify({
	afterOne: afterOne, afterBoth: afterBoth, single: single, sendsAfterReconcile: sendsAfterReconcile
}));
`)
	var got struct {
		AfterOne            []string `json:"afterOne"`
		AfterBoth           []string `json:"afterBoth"`
		Single              []string `json:"single"`
		SendsAfterReconcile int      `json:"sendsAfterReconcile"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.AfterOne, []string{"children.0"}) {
		t.Fatalf("after s:[1] = %v, want [children.0]", got.AfterOne)
	}
	if !reflect.DeepEqual(got.AfterBoth, []string{"children.0", "children.1"}) {
		t.Fatalf("after s:[1,2] = %v, want both", got.AfterBoth)
	}
	if !reflect.DeepEqual(got.Single, []string{"children.1"}) {
		t.Fatalf("single-select reconcile = %v, want [children.1]", got.Single)
	}
	// Reconciles must not echo back to the server.
	if got.SendsAfterReconcile != 0 {
		t.Fatalf("reconcile produced %d outbound frames, want 0", got.SendsAfterReconcile)
	}
}

func TestJawstreeJS_OnSelectionChangeSendsDelta(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }, { id: "children.1", name: "b" }] };
jawstreeInit({ key: "k", jid: "Jid.1", options: (1<<2), data: data });
var t = window["jawstree_k"];
// Simulate a user selecting children.0 (preorder index 1): Quercus mutates and fires
// onSelectionChange, which the adapter turns into a delta frame.
t.selectNodeById("children.0", true);
process.stdout.write(JSON.stringify({ sends: sends }));
`)
	var got struct {
		Sends []string `json:"sends"`
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
}

func TestJawstreeJS_UpdatesAreScopedByKey(t *testing.T) {
	raw := runJawstreeJSSnippet(t, jsMock+`
var data = { children: [{ id: "children.0", name: "a" }] };
jawstreeInit({ key: "one", jid: "Jid.1", options: (1<<2), data: data });
jawstreeInit({ key: "two", jid: "Jid.1", options: (1<<2), data: data });
// A selection update for "one" must not touch "two".
jawstreeSelection({ key: "one", s: [1] });
process.stdout.write(JSON.stringify({
	one: window["jawstree_one"].getSelectedNodes().map(function (n) { return n.id; }),
	two: window["jawstree_two"].getSelectedNodes().map(function (n) { return n.id; })
}));
`)
	var got struct {
		One []string `json:"one"`
		Two []string `json:"two"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.One, []string{"children.0"}) {
		t.Fatalf("tree one = %v, want [children.0]", got.One)
	}
	if len(got.Two) != 0 {
		t.Fatalf("tree two leaked selection: %v", got.Two)
	}
}
