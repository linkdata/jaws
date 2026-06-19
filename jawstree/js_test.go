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

type jsSetPathCall struct {
	ID  string `json:"id"`
	Set bool   `json:"set"`
}

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
