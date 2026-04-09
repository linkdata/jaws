package assets

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/staticserve"
)

func Test_PreloadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	const extraImage = "favicon.png"
	const extraFont = "someExtraFont.woff2"

	serveJS, err := staticserve.New("/jaws/.jaws.js", JavascriptText)
	if err != nil {
		t.Fatal(err)
	}

	txt, fav := PreloadHTML()
	if strings.Contains(txt, serveJS.Name) {
		t.Fatalf("unexpected preload output contains %q: %q", serveJS.Name, txt)
	}
	if strings.Count(txt, "<script>") != strings.Count(txt, "</script>") {
		t.Fatalf("script tags are unbalanced: %q", txt)
	}
	if fav != "" {
		t.Fatalf("unexpected favicon %q", fav)
	}

	mustParseURL := func(urlstr string) *url.URL {
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	txt, fav = PreloadHTML(
		mustParseURL(serveJS.Name),
		mustParseURL(extraScript),
		mustParseURL(extraStyle),
		mustParseURL(extraImage),
		mustParseURL(extraFont),
	)
	if !strings.Contains(txt, serveJS.Name) {
		t.Fatalf("missing %q in preload output: %q", serveJS.Name, txt)
	}
	if !strings.Contains(txt, extraScript) {
		t.Fatalf("missing %q in preload output: %q", extraScript, txt)
	}
	if !strings.Contains(txt, extraStyle) {
		t.Fatalf("missing %q in preload output: %q", extraStyle, txt)
	}
	if !strings.Contains(txt, extraImage) {
		t.Fatalf("missing %q in preload output: %q", extraImage, txt)
	}
	if !strings.Contains(txt, extraFont) {
		t.Fatalf("missing %q in preload output: %q", extraFont, txt)
	}
	if strings.Count(txt, "<script") != strings.Count(txt, "</script>") {
		t.Fatalf("script tags are unbalanced: %q", txt)
	}
	if fav != extraImage {
		t.Fatalf("favicon = %q, want %q", fav, extraImage)
	}
}

func TestJawsKeyString(t *testing.T) {
	if got := JawsKeyString(0); got != "" {
		t.Fatalf("JawsKeyString(0) = %q, want empty", got)
	}
	if got := JawsKeyString(1); got != "1" {
		t.Fatalf("JawsKeyString(1) = %q, want %q", got, "1")
	}
}

func TestJawsKeyValue(t *testing.T) {
	tests := []struct {
		name    string
		jawsKey string
		want    uint64
	}{
		{name: "blank", jawsKey: "", want: 0},
		{name: "1", jawsKey: "1", want: 1},
		{name: "-1", jawsKey: "-1", want: 0},
		{name: "2/", jawsKey: "2/", want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JawsKeyValue(tt.jawsKey); got != tt.want {
				t.Errorf("JawsKeyValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func runJawsJSSnippet(t *testing.T, snippet string) string {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node executable not available")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	jsPath := filepath.Join(filepath.Dir(file), "jaws.js")

	script := `
const fs = require("fs");
const src = fs.readFileSync(process.argv[1], "utf8");

global.window = {
	location: { protocol: "http:", host: "example.test", reload: function(){}, assign: function(){} },
	addEventListener: function(){},
	jawsNames: {},
};
global.document = {
	readyState: "loading",
	addEventListener: function(){},
	querySelector: function(selector){
		if (selector === 'meta[name="jawsKey"]') {
			return { content: "123" };
		}
		return null;
	},
	querySelectorAll: function(){ return { forEach: function(){} }; },
	getElementById: function(){ return null; },
};
global.XMLHttpRequest = function(){};
global.Event = function(){};
global.Node = function(){};
global.WebSocket = function(){};

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

func TestJawsJS_JsVarNestedPathUsesTopLevelNameRouting(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;

window.app = { state: 0 };
window.jawsNames["app"] = "Jid.9";
jaws = new FakeSocket();

jawsVar("app.state", 42);
process.stdout.write(jaws.sent[0] || "");
`)

	if raw == "" {
		t.Fatal("jawsVar did not emit a websocket frame")
	}

	msg, ok := wire.Parse([]byte(raw))
	if !ok {
		t.Fatalf("Set frame must be parseable by jawswire.Parse, got %q", raw)
	}
	if msg.What != what.Set {
		t.Fatalf("unexpected what: got %v", msg.What)
	}
	if msg.Jid != 9 {
		t.Fatalf("nested JsVar path should route through top-level name registration, got %v in %q", msg.Jid, raw)
	}
	if msg.Data != "state=42" {
		t.Fatalf("unexpected Set payload %q", msg.Data)
	}
}

func TestJawsJS_RemoveFromNonManagedContainerIsInvalidAndDroppedByParser(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const topElem = {
	id: "container",
	querySelectorAll: function() {
		return [{ id: "Jid.1" }, { id: "Jid.2" }];
	}
};
jawsRemoving(topElem);
process.stdout.write(jaws.sent[0] || "");
`)

	if raw == "" {
		t.Fatal("jawsRemoving did not emit a websocket frame")
	}

	if msg, ok := wire.Parse([]byte(raw)); ok {
		t.Fatalf("expected invalid untrusted Remove frame to be dropped by parser, got %+v from %q", msg, raw)
	}
}

func TestJawsJS_JsVarWithoutRegisteredTopLevelNameDoesNotEmitInvalidFrame(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;

window.app = { state: 0 };
jaws = new FakeSocket();

jawsVar("app.state", 42);
process.stdout.write(jaws.sent[0] || "");
`)

	if raw != "" {
		if _, ok := wire.Parse([]byte(raw)); !ok {
			t.Fatalf("jawsVar should not emit unparseable Set frame when JsVar name is unregistered, got %q", raw)
		}
	}
}

func TestJawsJS_SetSkipsUnchangedJsVarUpdate(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let setCalls = 0;
let currentState = 7;
window.app = {};
Object.defineProperty(window.app, "state", {
	get: function() { return currentState; },
	set: function(v) { setCalls++; currentState = v; },
	enumerable: true,
	configurable: true,
});

const elem = { id: "Jid.1", dataset: { jawsname: "app" } };
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Set", "Jid.1", "state=7");
jawsPerform("Set", "Jid.1", "state=8");

process.stdout.write(JSON.stringify({ setCalls: setCalls, state: window.app.state }));
`)

	var got struct {
		SetCalls int `json:"setCalls"`
		State    int `json:"state"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.SetCalls != 1 {
		t.Fatalf("Set() writes = %d, want 1", got.SetCalls)
	}
	if got.State != 8 {
		t.Fatalf("state = %d, want 8", got.State)
	}
}

func TestJawsJS_ValueSkipsUnchangedCheckboxUpdate(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let checked = true;
let checkedWrites = 0;
const elem = {
	id: "Jid.1",
	tagName: "INPUT",
	getAttribute: function(name) {
		if (name === "type") return "checkbox";
		return null;
	}
};
Object.defineProperty(elem, "checked", {
	get: function() { return checked; },
	set: function(v) { checkedWrites++; checked = v; },
	enumerable: true,
	configurable: true,
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Value", "Jid.1", JSON.stringify("true"));
jawsPerform("Value", "Jid.1", JSON.stringify("false"));

process.stdout.write(JSON.stringify({ checkedWrites: checkedWrites, checked: checked }));
`)

	var got struct {
		CheckedWrites int  `json:"checkedWrites"`
		Checked       bool `json:"checked"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.CheckedWrites != 1 {
		t.Fatalf("checkbox writes = %d, want 1", got.CheckedWrites)
	}
	if got.Checked {
		t.Fatalf("checkbox final value = %v, want false", got.Checked)
	}
}

func TestJawsJS_SetAttrSkipsUnchangedAttributeValue(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let attrWrites = 0;
const elem = {
	id: "Jid.1",
	attrs: { title: "same" },
	getAttribute: function(name) {
		if (Object.prototype.hasOwnProperty.call(this.attrs, name)) {
			return this.attrs[name];
		}
		return null;
	},
	setAttribute: function(name, value) {
		attrWrites++;
		this.attrs[name] = value;
	}
};
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("SAttr", "Jid.1", JSON.stringify("title\nsame"));
jawsPerform("SAttr", "Jid.1", JSON.stringify("title\nchanged"));

process.stdout.write(JSON.stringify({ attrWrites: attrWrites, title: elem.attrs.title }));
`)

	var got struct {
		AttrWrites int    `json:"attrWrites"`
		Title      string `json:"title"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.AttrWrites != 1 {
		t.Fatalf("attribute writes = %d, want 1", got.AttrWrites)
	}
	if got.Title != "changed" {
		t.Fatalf("title = %q, want %q", got.Title, "changed")
	}
}

func TestJawsJS_DebugEnabledWhenMetaTagIsPresent(t *testing.T) {
	raw := runJawsJSSnippet(t, `
document.querySelector = function(selector) {
	if (selector === 'meta[name="jawsDebug"]') {
		return { content: "" };
	}
	if (selector === 'meta[name="jawsKey"]') {
		return { content: "123" };
	}
	return null;
};
jawsDebug = false;
function FakeSocket() {}
FakeSocket.prototype.addEventListener = function() {};
WebSocket = FakeSocket;
jawsConnect();
process.stdout.write(JSON.stringify({ jawsDebug: jawsDebug }));
`)

	var got struct {
		JawsDebug bool `json:"jawsDebug"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.JawsDebug {
		t.Fatal("jawsDebug = false, want true when meta[name=\"jawsDebug\"] is present")
	}
}

func TestJawsJS_InnerComparesAndUpdatesWhenDebugDisabled(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = false;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
let innerReads = 0;
let innerWrites = 0;
let innerValue = "<i>old</i>";
const elem = {
	id: "Jid.1",
	querySelectorAll: function() { return []; }
};
Object.defineProperty(elem, "innerHTML", {
	get: function() {
		innerReads++;
		return innerValue;
	},
	set: function(v) {
		innerWrites++;
		innerValue = v;
	},
	enumerable: true,
	configurable: true,
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Inner", "Jid.1", JSON.stringify("<b>x</b>"));
process.stdout.write(JSON.stringify({ innerReads: innerReads, innerWrites: innerWrites, html: innerValue, warningCount: warnings.length }));
`)

	var got struct {
		InnerReads   int    `json:"innerReads"`
		InnerWrites  int    `json:"innerWrites"`
		HTML         string `json:"html"`
		WarningCount int    `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.InnerReads != 1 {
		t.Fatalf("innerHTML reads = %d, want 1", got.InnerReads)
	}
	if got.InnerWrites != 1 {
		t.Fatalf("innerHTML writes = %d, want 1", got.InnerWrites)
	}
	if got.HTML != "<b>x</b>" {
		t.Fatalf("innerHTML = %q, want %q", got.HTML, "<b>x</b>")
	}
	if got.WarningCount != 0 {
		t.Fatalf("warnings = %d, want 0", got.WarningCount)
	}
}

func TestJawsJS_ReplaceComparesAndUpdatesWhenDebugDisabled(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = false;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
let outerReads = 0;
let replaceCalls = 0;
let outerValue = "<div id=\"Jid.1\"><i>old</i></div>";
const elem = {
	id: "Jid.1",
	querySelectorAll: function() { return []; },
	replaceWith: function() { replaceCalls++; }
};
Object.defineProperty(elem, "outerHTML", {
	get: function() {
		outerReads++;
		return outerValue;
	},
	enumerable: true,
	configurable: true,
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };
document.createElement = function(tag) {
	if (tag !== "template") throw new Error("unexpected tag " + tag);
	const template = {
		content: {
			querySelectorAll: function() { return []; }
		}
	};
	Object.defineProperty(template, "innerHTML", {
		get: function() { return this._inner || ""; },
		set: function(v) { this._inner = v; },
		enumerable: true,
		configurable: true,
	});
	return template;
};

jawsPerform("Replace", "Jid.1", JSON.stringify("<div id=\"Jid.1\"></div>"));
process.stdout.write(JSON.stringify({ outerReads: outerReads, replaceCalls: replaceCalls, warningCount: warnings.length }));
`)

	var got struct {
		OuterReads   int `json:"outerReads"`
		ReplaceCalls int `json:"replaceCalls"`
		WarningCount int `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.OuterReads != 1 {
		t.Fatalf("outerHTML reads = %d, want 1", got.OuterReads)
	}
	if got.ReplaceCalls != 1 {
		t.Fatalf("replace calls = %d, want 1", got.ReplaceCalls)
	}
	if got.WarningCount != 0 {
		t.Fatalf("warnings = %d, want 0", got.WarningCount)
	}
}

func TestJawsJS_InnerWarnsWhenDebugEnabledAndHTMLUnchanged(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = true;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
const elem = {
	id: "Jid.1",
	querySelectorAll: function() { return []; },
	innerHTML: "<b>x</b>"
};
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Inner", "Jid.1", JSON.stringify("<b>x</b>"));
process.stdout.write(JSON.stringify({ warnings: warnings, innerHTML: elem.innerHTML }));
`)

	var got struct {
		Warnings  []string `json:"warnings"`
		InnerHTML string   `json:"innerHTML"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got.Warnings))
	}
	if !strings.Contains(got.Warnings[0], "marked dirty but it generated the same HTML") {
		t.Fatalf("unexpected warning text %q", got.Warnings[0])
	}
	if got.InnerHTML != "<b>x</b>" {
		t.Fatalf("innerHTML = %q, want unchanged", got.InnerHTML)
	}
}

func TestJawsJS_ReplaceWarnsWhenDebugEnabledAndHTMLUnchanged(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = true;
let replaceCalls = 0;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
const elem = {
	id: "Jid.1",
	querySelectorAll: function() { return []; },
	outerHTML: "<div id=\"Jid.1\"></div>",
	replaceWith: function() { replaceCalls++; }
};
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };
document.createElement = function(tag) {
	if (tag !== "template") throw new Error("unexpected tag " + tag);
	const template = {
		content: {
			querySelectorAll: function() { return []; }
		}
	};
	Object.defineProperty(template, "innerHTML", {
		get: function() { return this._inner || ""; },
		set: function(v) { this._inner = v; },
		enumerable: true,
		configurable: true,
	});
	return template;
};

jawsPerform("Replace", "Jid.1", JSON.stringify("<div id=\"Jid.1\"></div>"));
process.stdout.write(JSON.stringify({ warnings: warnings, replaceCalls: replaceCalls }));
`)

	var got struct {
		Warnings     []string `json:"warnings"`
		ReplaceCalls int      `json:"replaceCalls"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got.Warnings))
	}
	if !strings.Contains(got.Warnings[0], "marked dirty but it generated the same HTML") {
		t.Fatalf("unexpected warning text %q", got.Warnings[0])
	}
	if got.ReplaceCalls != 0 {
		t.Fatalf("replace calls = %d, want 0", got.ReplaceCalls)
	}
}
