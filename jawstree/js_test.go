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
		{ tree: "tree", id: "children.0", set: false }
	),
	deselectUnselected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: false },
		["children.1"],
		{ tree: "tree", id: "children.0", set: false }
	),
	selectAlreadySelected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: false },
		["children.0"],
		{ tree: "tree", id: "children.0", set: true }
	),
	multiDeselectUnselected: run(
		{ multiSelectEnabled: true, cascadeSelectChildren: false },
		[],
		{ tree: "tree", id: "children.0", set: false }
	),
	cascadeDeselectSelected: run(
		{ multiSelectEnabled: false, cascadeSelectChildren: true },
		["children.0"],
		{ tree: "tree", id: "children.0", set: false }
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

jawstreeSetPath({ tree: "tree", id: "parent", set: true });
jawstreeSetPath({ tree: "tree", id: "child", set: true });
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
	window["jawstree_tree"] = jawstreeNew("tree", root, `+strconv.Itoa(int(CascadeSelectChildren))+`);

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

window["jawstree_tree"] = jawstreeNew("tree", root, `+strconv.Itoa(int(CascadeSelectChildren))+`);
jawstreeSet({ tree: "tree", data: root });

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
