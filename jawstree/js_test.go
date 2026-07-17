package jawstree

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

type jsSetPathCall struct {
	ID  string `json:"id"`
	Set bool   `json:"set"`
}

type jsVarCall struct {
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

// Asset files are already tracked by git. Keep these tests focused on browser
// adapter behavior; do not add stored-hash provenance tests for files whose
// contents and history are in the repository.

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

func TestJawstreeJS_InitUsesPreloadedRoot(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
const root = { children: [{ id: "children.0", name: "Documents" }] };
let got = null;
const initialized = { ready: true };
const container = { hidden: true };
global.document = {
	getElementById: function(id) {
		return id === "Jid.7" ? container : null;
	}
};
window["jawstreeroot_private"] = root;
jawstreeNew = function(key, jid, data, options) {
	got = { key: key, jid: jid, data: data, options: options };
	return initialized;
};

jawstreeInit({ key: "private", jid: "Jid.7", options: 3 });
process.stdout.write(JSON.stringify({
	got: got,
	usesRoot: got.data === root,
	stored: window["jawstree_private"] === initialized,
	visible: !container.hidden
}));
`)

	var got struct {
		Got struct {
			Key     string `json:"key"`
			Jid     string `json:"jid"`
			Options int    `json:"options"`
		} `json:"got"`
		UsesRoot bool `json:"usesRoot"`
		Stored   bool `json:"stored"`
		Visible  bool `json:"visible"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if got.Got.Key != "private" || got.Got.Jid != "Jid.7" || got.Got.Options != 3 {
		t.Fatalf("initializer arguments = %+v, want key=private jid=Jid.7 options=3", got.Got)
	}
	if !got.UsesRoot {
		t.Fatal("initializer did not use the root seeded by the hidden JsVar")
	}
	if !got.Stored {
		t.Fatal("initializer did not store the constructed tree on window")
	}
	if !got.Visible {
		t.Fatal("initializer did not unhide the managed Jid container")
	}
}

func TestJawstreeJS_SetPathSingleSelectDeselect(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
function run(options, selected, arg) {
	window["jawstree_tree"] = {
		options: options,
		calls: [],
		getSelectedNodes: function() {
			return selected.map(function(id) { return { id: id }; });
		},
		selectNodeById: function(id, set) {
			this.calls.push({ id: id, set: set });
		}
	};
	jawstreeSetPath(arg);
	return window["jawstree_tree"].calls;
}

process.stdout.write(JSON.stringify({
	deselectSelected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: false },
		["children.0"],
		{ key: "tree", id: "children.0", set: false }
	),
	deselectUnselected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: false },
		["children.1"],
		{ key: "tree", id: "children.0", set: false }
	),
	selectAlreadySelected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: false },
		["children.0"],
		{ key: "tree", id: "children.0", set: true }
	),
	multiDeselectUnselected: run(
		{ multiSelectEnabled: true, cascadeSelectChildren: false },
		[],
		{ key: "tree", id: "children.0", set: false }
	),
	cascadeDeselectSelected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: true },
		["children.0"],
		{ key: "tree", id: "children.0", set: false }
	)
}));
`)

	var got struct {
		DeselectSelected      []jsSetPathCall `json:"deselectSelected"`
		DeselectUnselected    []jsSetPathCall `json:"deselectUnselected"`
		SelectAlreadySelected []jsSetPathCall `json:"selectAlreadySelected"`
		MultiDeselectUnsel    []jsSetPathCall `json:"multiDeselectUnselected"`
		CascadeDeselectSel    []jsSetPathCall `json:"cascadeDeselectSelected"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}

	wantDeselect := []jsSetPathCall{{ID: "children.0", Set: false}}
	if !reflect.DeepEqual(got.DeselectSelected, wantDeselect) {
		t.Fatalf("deselectSelected = %#v, want %#v", got.DeselectSelected, wantDeselect)
	}
	if len(got.DeselectUnselected) != 0 {
		t.Fatalf("deselectUnselected = %#v, want no call", got.DeselectUnselected)
	}
	if len(got.SelectAlreadySelected) != 0 {
		t.Fatalf("selectAlreadySelected = %#v, want no call", got.SelectAlreadySelected)
	}
	if !reflect.DeepEqual(got.MultiDeselectUnsel, wantDeselect) {
		t.Fatalf("multiDeselectUnselected = %#v, want %#v", got.MultiDeselectUnsel, wantDeselect)
	}
	if !reflect.DeepEqual(got.CascadeDeselectSel, wantDeselect) {
		t.Fatalf("cascadeDeselectSelected = %#v, want %#v", got.CascadeDeselectSel, wantDeselect)
	}
}

func TestJawstreeJS_SetPathSingleSelectCascadeReplay(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
window["jawstree_tree"] = {
	options: { multiSelectEnabled: false, cascadeSelectChildren: true },
	selected: [],
	descendants: { "parent": ["child"] },
	getSelectedNodes: function() {
		return this.selected.map(function(id) { return { id: id }; });
	},
	selectNodeById: function(id, set) {
		if (set) {
			this.selected = [id].concat(this.descendants[id] || []);
			return;
		}
		var remove = [id].concat(this.descendants[id] || []);
		this.selected = this.selected.filter(function(selectedID) {
			return remove.indexOf(selectedID) == -1;
		});
	}
};

jawstreeSetPath({ key: "tree", id: "parent", set: true });
jawstreeSetPath({ key: "tree", id: "child", set: true });
process.stdout.write(JSON.stringify(window["jawstree_tree"].selected));
`)

	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	want := []string{"parent", "child"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected = %#v, want %#v", got, want)
	}
}

func TestJawstreeJS_UpdatesAreScopedByKey(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
function makeTree() {
	return {
		options: { multiSelectEnabled: true, cascadeSelectChildren: false },
		sets: [],
		selects: [],
		jawsApplyingSet: false,
		setData: function(data) { this.sets.push(data.length); },
		getSelectedNodes: function() { return []; },
		selectNodeById: function(id, set) { this.selects.push({ id: id, set: set }); }
	};
}

window["jawstreeroot_one"] = { marker: "old-one" };
window["jawstreeroot_two"] = { marker: "old-two" };
window["jawstree_one"] = makeTree();
window["jawstree_two"] = makeTree();

const next = { marker: "new-one" };
jawstreeSet({ key: "one", data: next });
jawstreeSetPath({ key: "two", id: "children.0", set: false });

process.stdout.write(JSON.stringify({
	oneSets: window["jawstree_one"].sets,
	oneSelects: window["jawstree_one"].selects,
	twoSets: window["jawstree_two"].sets,
	twoSelects: window["jawstree_two"].selects,
	oneRoot: window["jawstreeroot_one"],
	twoRoot: window["jawstreeroot_two"]
}));
`)

	var got struct {
		OneSets    []int             `json:"oneSets"`
		OneSelects []jsSetPathCall   `json:"oneSelects"`
		TwoSets    []int             `json:"twoSets"`
		TwoSelects []jsSetPathCall   `json:"twoSelects"`
		OneRoot    map[string]string `json:"oneRoot"`
		TwoRoot    map[string]string `json:"twoRoot"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.OneSets, []int{0}) || len(got.OneSelects) != 0 {
		t.Fatalf("first tree calls = sets %#v selects %#v", got.OneSets, got.OneSelects)
	}
	if len(got.TwoSets) != 0 || !reflect.DeepEqual(got.TwoSelects, []jsSetPathCall{{ID: "children.0", Set: false}}) {
		t.Fatalf("second tree calls = sets %#v selects %#v", got.TwoSets, got.TwoSelects)
	}
	if got.OneRoot["marker"] != "new-one" || got.TwoRoot["marker"] != "old-two" {
		t.Fatalf("roots crossed keys: one=%#v two=%#v", got.OneRoot, got.TwoRoot)
	}
}

func TestJawstreeJS_NewSingleSelectCascadeSnapshot(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
function collectDescendantIDs(node, ids) {
		if (node.children) {
			node.children.forEach(function(child) {
				ids.push(child.id);
				collectDescendantIDs(child, ids);
			});
		}
	}

	function Treeview(options) {
		this.options = options;
		this.nodes = {};
		this.selected = [];
		this.setData(options.data);
	}

	Treeview.prototype.getSelectedNodes = function() {
		return this.selected.map(function(id) { return { id: id }; });
	};

	Treeview.prototype.selectNodeById = function(id, set) {
		var node = this.nodes[id];
		if (!node) {
			return;
		}
		if (set && this.options.cascadeSelectChildren && !this.options.multiSelectEnabled) {
			var ids = [id];
			collectDescendantIDs(node, ids);
			this.selected = ids;
			return;
		}
		if (set) {
			if (this.selected.indexOf(id) == -1) {
				this.selected.push(id);
			}
			return;
		}
		this.selected = this.selected.filter(function(selectedID) { return selectedID != id; });
	};

	Treeview.prototype.setData = function(data) {
		var selectedIDs = [];
		var self = this;
		this.nodes = {};
		this.selected = [];
		function walk(node) {
			self.nodes[node.id] = node;
			if (node.selected) {
				selectedIDs.push(node.id);
			}
			if (node.children) {
				node.children.forEach(walk);
			}
		}
		data.forEach(walk);
		selectedIDs.forEach(function(id) {
			self.selectNodeById(id, true);
			self.options.onSelectionChange(self.getSelectedNodes());
		});
	};

	global.Treeview = Treeview;
	var calls = [];
	global.jawsVar = function(path, value) {
		calls.push({ path: path, value: value });
	};

	var root = { children: [{
		id: "parent",
		name: "parent",
		selected: true,
		children: [{
			id: "child",
			name: "child",
			selected: true,
			children: []
		}]
	}]};

	window["jawstreeroot_tree"] = root;
	window["jawstree_tree"] = jawstreeNew("tree", "Jid.1", root, `+strconv.Itoa(int(CascadeSelectChildren))+`);

	process.stdout.write(JSON.stringify({
		selected: window["jawstree_tree"].selected,
		parentSelected: window["jawstreeroot_tree"].children[0].selected,
		childSelected: window["jawstreeroot_tree"].children[0].children[0].selected,
		calls: calls
	}));
	`)

	var got struct {
		Selected       []string    `json:"selected"`
		ParentSelected bool        `json:"parentSelected"`
		ChildSelected  bool        `json:"childSelected"`
		Calls          []jsVarCall `json:"calls"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	want := []string{"parent", "child"}
	if !reflect.DeepEqual(got.Selected, want) {
		t.Fatalf("selected = %#v, want %#v", got.Selected, want)
	}
	if !got.ParentSelected || !got.ChildSelected {
		t.Fatalf("root selection changed: parent=%v child=%v, want both true", got.ParentSelected, got.ChildSelected)
	}
	if len(got.Calls) != 0 {
		t.Fatalf("jawsVar calls during jawstreeNew = %#v, want none", got.Calls)
	}
}

func TestJawstreeJS_SetSingleSelectCascadeSnapshot(t *testing.T) {
	raw := runJawstreeJSSnippet(t, `
function collectDescendantIDs(node, ids) {
	if (node.children) {
		node.children.forEach(function(child) {
			ids.push(child.id);
			collectDescendantIDs(child, ids);
		});
	}
}

function Treeview(options) {
	this.options = options;
	this.nodes = {};
	this.selected = [];
}

Treeview.prototype.getSelectedNodes = function() {
	return this.selected.map(function(id) { return { id: id }; });
};

Treeview.prototype.selectNodeById = function(id, set) {
	var node = this.nodes[id];
	if (!node) {
		return;
	}
	if (set && this.options.cascadeSelectChildren && !this.options.multiSelectEnabled) {
		var ids = [id];
		collectDescendantIDs(node, ids);
		this.selected = ids;
		return;
	}
	if (set) {
		if (this.selected.indexOf(id) == -1) {
			this.selected.push(id);
		}
		return;
	}
	this.selected = this.selected.filter(function(selectedID) { return selectedID != id; });
};

Treeview.prototype.setData = function(data) {
	var selectedIDs = [];
	var self = this;
	this.nodes = {};
	this.selected = [];
	function walk(node) {
		self.nodes[node.id] = node;
		if (node.selected) {
			selectedIDs.push(node.id);
		}
		if (node.children) {
			node.children.forEach(walk);
		}
	}
	data.forEach(walk);
	selectedIDs.forEach(function(id) {
		self.selectNodeById(id, true);
		self.options.onSelectionChange(self.getSelectedNodes());
	});
};

global.Treeview = Treeview;
var calls = [];
global.jawsVar = function(path, value) {
	calls.push({ path: path, value: value });
};

var root = { children: [{
	id: "parent",
	name: "parent",
	selected: true,
	children: [{
		id: "child",
		name: "child",
		selected: true,
		children: []
	}]
}]};

window["jawstree_tree"] = jawstreeNew("tree", "Jid.1", root, `+strconv.Itoa(int(CascadeSelectChildren))+`);
jawstreeSet({ key: "tree", data: root });

process.stdout.write(JSON.stringify({
	selected: window["jawstree_tree"].selected,
	parentSelected: window["jawstreeroot_tree"].children[0].selected,
	childSelected: window["jawstreeroot_tree"].children[0].children[0].selected,
	calls: calls
}));
`)

	var got struct {
		Selected       []string    `json:"selected"`
		ParentSelected bool        `json:"parentSelected"`
		ChildSelected  bool        `json:"childSelected"`
		Calls          []jsVarCall `json:"calls"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	want := []string{"parent", "child"}
	if !reflect.DeepEqual(got.Selected, want) {
		t.Fatalf("selected = %#v, want %#v", got.Selected, want)
	}
	if !got.ParentSelected || !got.ChildSelected {
		t.Fatalf("root selection changed: parent=%v child=%v, want both true", got.ParentSelected, got.ChildSelected)
	}
	if len(got.Calls) != 0 {
		t.Fatalf("jawsVar calls during jawstreeSet = %#v, want none", got.Calls)
	}
}
