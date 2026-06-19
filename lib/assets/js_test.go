package assets

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/url"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/staticserve"
)

// Asset files are already tracked by git. Keep these tests focused on generated
// HTML and browser behavior; do not add stored-hash provenance tests for files
// whose contents and history are in the repository.

func Test_PreloadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraScriptWithQuery = "someExtraQuery.js?x=1&copy=2"
	const extraStyle = "someExtraStyle.css"
	const extraImage = "favicon.png"
	const extraFont = "someExtraFont.woff2"
	const extraFontWithQuery = "someExtraFontQuery.woff2?x=1&copy=2"

	serveJS, err := staticserve.New("/jaws/.jaws.js", JavascriptText)
	if err != nil {
		t.Fatal(err)
	}

	txt, fav := PreloadHTML()
	if strings.Contains(txt, serveJS.Name) {
		t.Fatalf("unexpected preload output contains %q: %q", serveJS.Name, txt)
	}
	// Count "<script" (the opening tag is emitted as "<script defer src=...>",
	// so the literal "<script>" never appears) to actually validate balance.
	if strings.Count(txt, "<script") != strings.Count(txt, "</script>") {
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
		mustParseURL(extraScriptWithQuery),
		mustParseURL(extraStyle),
		mustParseURL(extraImage),
		mustParseURL(extraFont),
		mustParseURL(extraFontWithQuery),
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
	if strings.Contains(txt, extraScriptWithQuery) || strings.Contains(txt, extraFontWithQuery) {
		t.Fatalf("preload output contains unescaped query ampersand: %q", txt)
	}
	if !strings.Contains(txt, `src="someExtraQuery.js?x=1&amp;copy=2"`) {
		t.Fatalf("missing escaped script query in preload output: %q", txt)
	}
	if !strings.Contains(txt, `href="someExtraFontQuery.woff2?x=1&amp;copy=2"`) {
		t.Fatalf("missing escaped font query in preload output: %q", txt)
	}
	if strings.Count(txt, "<script") != strings.Count(txt, "</script>") {
		t.Fatalf("script tags are unbalanced: %q", txt)
	}

	// Assert the full as/type structure, not just substring presence. Compute the
	// expected MIME types the same way PreloadHTML does so the test stays correct
	// regardless of the platform's MIME table.
	fontMime, _, _ := strings.Cut(mime.TypeByExtension(".woff2"), ";")
	var wantFontLink string
	if strings.HasPrefix(fontMime, "font") {
		wantFontLink = `<link rel="preload" href="someExtraFont.woff2" as="font" type="` + fontMime + `">`
	} else {
		// No font/* MIME on this platform: still a preload link, but no as/type.
		wantFontLink = `<link rel="preload" href="someExtraFont.woff2">`
	}
	if !strings.Contains(txt, wantFontLink) {
		t.Fatalf("missing structured font preload %q in %q", wantFontLink, txt)
	}

	pngMime, _, _ := strings.Cut(mime.TypeByExtension(".png"), ";")
	wantFaviconLink := `<link rel="icon" type="` + pngMime + `" href="favicon.png">`
	if !strings.Contains(txt, wantFaviconLink) {
		t.Fatalf("missing structured favicon link %q in %q", wantFaviconLink, txt)
	}

	if fav != extraImage {
		t.Fatalf("favicon = %q, want %q", fav, extraImage)
	}
}

// Test_PreloadHTML_MultipleFaviconsLastWins pins the documented contract that when
// several resources qualify as favicons, only the last is honored (returned as
// faviconURL and emitted as the rel="icon" link) and the earlier ones are discarded
// entirely rather than emitted as preload links.
func Test_PreloadHTML_MultipleFaviconsLastWins(t *testing.T) {
	mustParseURL := func(urlstr string) *url.URL {
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	txt, fav := PreloadHTML(
		mustParseURL("favicon.png"),
		mustParseURL("favicon-dark.png"),
	)
	if fav != "favicon-dark.png" {
		t.Errorf("favicon = %q, want last-wins %q", fav, "favicon-dark.png")
	}
	if !strings.Contains(txt, `href="favicon-dark.png"`) {
		t.Errorf("winning favicon should be emitted as the icon link; got %q", txt)
	}
	if strings.Contains(txt, "favicon.png") {
		t.Errorf("earlier favicon should be discarded, not emitted anywhere; got %q", txt)
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

func TestJawsJS_JsVarNestedPathHandlesShadowedHasOwnProperty(t *testing.T) {
	raw := runJawsJSSnippet(t, `
	function FakeSocket() { this.readyState = 1; this.sent = []; }
	FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
	WebSocket = FakeSocket;

	window.app = { hasOwnProperty: 1, state: { value: 0 } };
	window.jawsNames["app"] = "Jid.9";
	jaws = new FakeSocket();

	jawsVar("app.state.value", 42);
	process.stdout.write(JSON.stringify({
		value: window.app.state.value,
		frame: jaws.sent[0] || ""
	}));
	`)

	var got struct {
		Value int    `json:"value"`
		Frame string `json:"frame"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if got.Value != 42 {
		t.Fatalf("jawsVar did not update shadowed object path: got %d", got.Value)
	}

	msg, ok := wire.Parse([]byte(got.Frame))
	if !ok {
		t.Fatalf("Set frame must be parseable by jawswire.Parse, got %q", got.Frame)
	}
	if msg.What != what.Set {
		t.Fatalf("unexpected what: got %v", msg.What)
	}
	if msg.Jid != 9 {
		t.Fatalf("nested JsVar path should route through top-level name registration, got %v in %q", msg.Jid, got.Frame)
	}
	if msg.Data != "state.value=42" {
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

func TestJawsJS_ClickIncludesCoordinatesAndRoute(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const parent = {
	id: "Jid.1",
	tagName: "DIV",
	getAttribute: function() { return null; },
	textContent: "",
	parentElement: null
};
const target = {
	id: "Jid.2",
	tagName: "DIV",
	getAttribute: function(name) { return name === "name" ? "save" : null; },
	textContent: "",
	parentElement: parent
};
const ev = new Event();
ev.target = target;
ev.clientX = 11.25;
ev.clientY = 22.5;
ev.shiftKey = true;
ev.ctrlKey = false;
ev.altKey = true;
ev.stopPropagation = function() {};

jawsClickHandler(ev);
process.stdout.write(jaws.sent[0] || "");
`)

	if raw == "" {
		t.Fatal("jawsClickHandler did not emit a websocket frame")
	}
	msg, ok := wire.Parse([]byte(raw))
	if !ok {
		t.Fatalf("click frame must be parseable by wire.Parse, got %q", raw)
	}
	if msg.What != what.Click {
		t.Fatalf("unexpected what: got %v", msg.What)
	}
	if msg.Data != "11.25 22.5 5 save\tJid.2\tJid.1" {
		t.Fatalf("unexpected click payload %q", msg.Data)
	}
}

func TestJawsJS_ClickInputOriginIgnored(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const parent = {
	id: "Jid.1",
	tagName: "DIV",
	getAttribute: function() { return null; },
	textContent: "",
	parentElement: null
};
const input = {
	id: "Jid.9",
	tagName: "INPUT",
	getAttribute: function(name) { return name === "name" ? "in" : null; },
	textContent: "",
	parentElement: parent
};
let stopped = false;
const ev = new Event();
ev.target = input;
ev.clientX = 7;
ev.clientY = 8;
ev.stopPropagation = function() { stopped = true; };

jawsClickHandler(ev);
process.stdout.write(JSON.stringify({ msg: jaws.sent[0] || "", stopped: stopped }));
`)

	var got struct {
		Msg     string `json:"msg"`
		Stopped bool   `json:"stopped"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Msg != "" {
		t.Fatalf("expected no frame for input-origin click, got %q", got.Msg)
	}
	if got.Stopped {
		t.Fatal("input-origin click should not be intercepted")
	}
}

func TestJawsJS_EventHandlersIgnoreConnectingSocket(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 0; this.sent = []; }
FakeSocket.prototype.send = function(msg) { throw new Error("send while connecting: " + msg); };
WebSocket = FakeSocket;

function run(handler, ev) {
	jaws = new FakeSocket();
	let prevented = false;
	let stopped = false;
	ev.preventDefault = function() { prevented = true; };
	ev.stopPropagation = function() { stopped = true; };
	handler(ev);
	return { sent: jaws.sent.length, prevented: prevented, stopped: stopped };
}

const target = {
	id: "Jid.1",
	tagName: "DIV",
	getAttribute: function(name) { return name === "name" ? "go" : null; },
	textContent: "",
	parentElement: null
};
const input = {
	id: "Jid.2",
	tagName: "INPUT",
	getAttribute: function(name) { return name === "type" ? "text" : null; },
	value: "typed",
	checked: false,
	selected: false,
	parentElement: null
};
const clickEv = new Event();
clickEv.target = target;
clickEv.clientX = 1;
clickEv.clientY = 2;
clickEv.shiftKey = false;
clickEv.ctrlKey = false;
clickEv.altKey = false;

const inputEv = new Event();
inputEv.currentTarget = input;

const contextEv = new Event();
contextEv.target = target;
contextEv.clientX = 3;
contextEv.clientY = 4;
contextEv.shiftKey = false;
contextEv.ctrlKey = false;
contextEv.altKey = false;

process.stdout.write(JSON.stringify({
	click: run(jawsClickHandler, clickEv),
	input: run(jawsInputHandler, inputEv),
	context: run(jawsContextMenuHandler, contextEv)
}));
`)

	var got struct {
		Click struct {
			Sent      int  `json:"sent"`
			Prevented bool `json:"prevented"`
			Stopped   bool `json:"stopped"`
		} `json:"click"`
		Input struct {
			Sent      int  `json:"sent"`
			Prevented bool `json:"prevented"`
			Stopped   bool `json:"stopped"`
		} `json:"input"`
		Context struct {
			Sent      int  `json:"sent"`
			Prevented bool `json:"prevented"`
			Stopped   bool `json:"stopped"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Click.Sent != 0 || got.Click.Prevented || got.Click.Stopped {
		t.Fatalf("connecting click handler should be inert, got %+v", got.Click)
	}
	if got.Input.Sent != 0 || got.Input.Prevented || got.Input.Stopped {
		t.Fatalf("connecting input handler should be inert, got %+v", got.Input)
	}
	if got.Context.Sent != 0 || got.Context.Prevented || got.Context.Stopped {
		t.Fatalf("connecting context menu handler should be inert, got %+v", got.Context)
	}
}

func TestJawsJS_SetValuePreservesTextSelection(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const input = {
	tagName: "INPUT",
	value: "hello",
	selectionStart: 1,
	selectionEnd: 5,
	getAttribute: function(name) { return name === "type" ? "text" : null; }
};

jawsSetValue(input, "say hello!");
process.stdout.write(JSON.stringify({
	value: input.value,
	start: input.selectionStart,
	end: input.selectionEnd
}));
`)

	var got struct {
		Value string `json:"value"`
		Start int    `json:"start"`
		End   int    `json:"end"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Value != "say hello!" || got.Start != 5 || got.End != 9 {
		t.Fatalf("unexpected text value/selection: %+v", got)
	}
}

func TestJawsJS_PerformRemoveAndReplaceMutateDOMAndReportRemovals(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const nodes = {};
function makeNode(id, tagName) {
	const node = new Node();
	node.id = id;
	node.tagName = tagName || "DIV";
	node.children = [];
	node.parentElement = null;
	node.querySelectorAll = function(selector) {
		if (selector === '[id^="' + jawsIdPrefix + '"]') {
			return this.children.filter(function(child) { return String(child.id || "").startsWith(jawsIdPrefix); });
		}
		return [];
	};
	node.removeChild = function(child) {
		this.children = this.children.filter(function(candidate) { return candidate !== child; });
		child.parentElement = null;
		delete nodes[child.id];
	};
	node.replaceWith = function(newNode) {
		const parent = this.parentElement;
		const idx = parent.children.indexOf(this);
		parent.children[idx] = newNode;
		newNode.parentElement = parent;
		delete nodes[this.id];
		nodes[newNode.id] = newNode;
	};
	nodes[id] = node;
	return node;
}

const parent = makeNode("Jid.1");
const child = makeNode("Jid.2");
const grandchild = makeNode("Jid.3");
child.children.push(grandchild);
grandchild.parentElement = child;
parent.children.push(child);
child.parentElement = parent;

const replaceParent = makeNode("parent");
const oldNode = makeNode("Jid.4");
replaceParent.children.push(oldNode);
oldNode.parentElement = replaceParent;

document.getElementById = function(id) { return nodes[id] || null; };
jawsElement = function(html) { return makeNode("Jid.5"); };
jawsAttachChildren = function(node) { return node; };

jawsPerform("Remove", "Jid.1", JSON.stringify("Jid.2"));
jawsPerform("Replace", "Jid.4", JSON.stringify('<div id="Jid.5"></div>'));

process.stdout.write(JSON.stringify({
	parentChildren: parent.children.map(function(child) { return child.id; }),
	replacedChildren: replaceParent.children.map(function(child) { return child.id; }),
	removeFrame: jaws.sent[0] || "",
	childStillRegistered: Boolean(nodes["Jid.2"]),
	oldStillRegistered: Boolean(nodes["Jid.4"])
}));
`)

	var got struct {
		ParentChildren       []string `json:"parentChildren"`
		ReplacedChildren     []string `json:"replacedChildren"`
		RemoveFrame          string   `json:"removeFrame"`
		ChildStillRegistered bool     `json:"childStillRegistered"`
		OldStillRegistered   bool     `json:"oldStillRegistered"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.ParentChildren) != 0 {
		t.Fatalf("Remove left children behind: %+v", got.ParentChildren)
	}
	if !reflect.DeepEqual(got.ReplacedChildren, []string{"Jid.5"}) {
		t.Fatalf("Replace children = %+v, want [Jid.5]", got.ReplacedChildren)
	}
	if got.ChildStillRegistered || got.OldStillRegistered {
		t.Fatalf("removed/replaced nodes still registered: %+v", got)
	}
	msg, ok := wire.Parse([]byte(got.RemoveFrame))
	if !ok {
		t.Fatalf("Remove frame must be parseable by wire.Parse, got %q", got.RemoveFrame)
	}
	if msg.What != what.Remove || msg.Jid != 2 || msg.Data != "Jid.3" {
		t.Fatalf("unexpected removal frame: %+v", msg)
	}
}

func TestJawsJS_FailedSocketPingsAndReloadsOnReconnect(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() {}
WebSocket = FakeSocket;
jaws = new FakeSocket();

let reloaded = 0;
window.location.reload = function() { reloaded++; };

function FakeXHR() {
	this.readyState = 0;
	this.status = 0;
	this.cb = null;
}
FakeXHR.prototype.open = function(method, url, async) {
	this.method = method;
	this.url = url;
	this.async = async;
};
FakeXHR.prototype.addEventListener = function(name, cb) {
	if (name === "readystatechange") {
		this.cb = cb;
	}
};
FakeXHR.prototype.send = function() {
	this.readyState = 4;
	this.status = 204;
	this.cb({ currentTarget: this });
};
XMLHttpRequest = FakeXHR;

jawsFailed();
process.stdout.write(JSON.stringify({
	jawsIsDate: jaws instanceof Date,
	reloaded: reloaded
}));
`)

	var got struct {
		JawsIsDate bool `json:"jawsIsDate"`
		Reloaded   int  `json:"reloaded"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.JawsIsDate || got.Reloaded != 1 {
		t.Fatalf("unexpected reconnect behavior: %+v", got)
	}
}

func TestJawsJS_ClickLeavesNonFiniteCoordinatesForServerValidation(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const target = {
	id: "Jid.2",
	tagName: "DIV",
	getAttribute: function(name) { return name === "name" ? "save" : null; },
	textContent: "",
	parentElement: null
};
const ev = new Event();
ev.clientX = Infinity;
ev.clientY = NaN;
ev.shiftKey = false;
ev.ctrlKey = false;
ev.altKey = false;

process.stdout.write(jawsBuildClickData(target, ev));
`)

	if raw != "Infinity NaN 0 save\tJid.2" {
		t.Fatalf("unexpected click payload %q", raw)
	}
}

func TestJawsJS_ClickHandlesNonElementTarget(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const parent = {
	id: "Jid.1",
	tagName: "DIV",
	getAttribute: function() { return null; },
	textContent: "",
	parentElement: null
};
const targetElem = {
	id: "Jid.2",
	tagName: "DIV",
	getAttribute: function(name) { return name === "name" ? "save" : null; },
	textContent: "",
	parentElement: parent
};
const textNodeLike = {
	parentElement: targetElem
};
const ev = new Event();
ev.target = textNodeLike;
ev.clientX = 11;
ev.clientY = 22;
ev.shiftKey = false;
ev.ctrlKey = false;
ev.altKey = false;
ev.stopPropagation = function() {};

jawsClickHandler(ev);
process.stdout.write(jaws.sent[0] || "");
`)

	if raw == "" {
		t.Fatal("jawsClickHandler did not emit a websocket frame")
	}
	msg, ok := wire.Parse([]byte(raw))
	if !ok {
		t.Fatalf("click frame must be parseable by wire.Parse, got %q", raw)
	}
	if msg.What != what.Click {
		t.Fatalf("unexpected what: got %v", msg.What)
	}
	if msg.Data != "11 22 0 save\tJid.2\tJid.1" {
		t.Fatalf("unexpected click payload %q", msg.Data)
	}
}

func TestJawsJS_ContextMenuIncludesCoordinatesAndSuppressesNativeMenu(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const parent = {
	id: "Jid.1",
	tagName: "DIV",
	getAttribute: function() { return null; },
	textContent: "",
	parentElement: null
};
const target = {
	id: "Jid.2",
	tagName: "DIV",
	getAttribute: function(name) { return name === "name" ? "menu" : null; },
	textContent: "",
	parentElement: parent
};
let prevented = false;
let stopped = false;
const ev = new Event();
ev.target = target;
ev.clientX = 33.25;
ev.clientY = 44.5;
ev.shiftKey = false;
ev.ctrlKey = true;
ev.altKey = false;
ev.stopPropagation = function() { stopped = true; };
ev.preventDefault = function() { prevented = true; };

jawsContextMenuHandler(ev);
process.stdout.write(JSON.stringify({ msg: jaws.sent[0] || "", prevented: prevented, stopped: stopped }));
`)

	var got struct {
		Msg       string `json:"msg"`
		Prevented bool   `json:"prevented"`
		Stopped   bool   `json:"stopped"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.Prevented {
		t.Fatal("expected context menu handler to call preventDefault")
	}
	if !got.Stopped {
		t.Fatal("expected context menu handler to call stopPropagation")
	}
	msg, ok := wire.Parse([]byte(got.Msg))
	if !ok {
		t.Fatalf("context menu frame must be parseable by wire.Parse, got %q", got.Msg)
	}
	if msg.What != what.ContextMenu {
		t.Fatalf("unexpected what: got %v", msg.What)
	}
	if msg.Data != "33.25 44.5 2 menu\tJid.2\tJid.1" {
		t.Fatalf("unexpected context menu payload %q", msg.Data)
	}
}

func TestJawsJS_ContextMenuInputOriginIgnored(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const input = {
	id: "Jid.9",
	tagName: "INPUT",
	getAttribute: function(name) { return name === "name" ? "in" : null; },
	textContent: "",
	parentElement: null
};
let prevented = false;
let stopped = false;
const ev = new Event();
ev.target = input;
ev.clientX = 7;
ev.clientY = 8;
ev.stopPropagation = function() { stopped = true; };
ev.preventDefault = function() { prevented = true; };

jawsContextMenuHandler(ev);
process.stdout.write(JSON.stringify({ msg: jaws.sent[0] || "", prevented: prevented, stopped: stopped }));
`)

	var got struct {
		Msg       string `json:"msg"`
		Prevented bool   `json:"prevented"`
		Stopped   bool   `json:"stopped"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Msg != "" {
		t.Fatalf("expected no frame for input-origin context menu, got %q", got.Msg)
	}
	if got.Prevented || got.Stopped {
		t.Fatalf("input-origin context menu should not be intercepted, got prevented=%v stopped=%v", got.Prevented, got.Stopped)
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

func TestJawsJS_ValueUpdatesTextareaLiveValue(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let value = "world";
let textContent = "default";
let valueWrites = 0;
let textContentWrites = 0;
const elem = {
	id: "Jid.1",
	tagName: "TEXTAREA",
	selectionStart: 5,
	selectionEnd: 5,
	getAttribute: function() { return null; }
};
Object.defineProperty(elem, "value", {
	get: function() { return value; },
	set: function(v) { valueWrites++; value = v; },
	enumerable: true,
	configurable: true,
});
Object.defineProperty(elem, "textContent", {
	get: function() { return textContent; },
	set: function(v) { textContentWrites++; textContent = v; },
	enumerable: true,
	configurable: true,
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Value", "Jid.1", JSON.stringify("hello world"));

process.stdout.write(JSON.stringify({
	value: value,
	textContent: textContent,
	valueWrites: valueWrites,
	textContentWrites: textContentWrites,
	selectionStart: elem.selectionStart,
	selectionEnd: elem.selectionEnd
}));
`)

	var got struct {
		Value             string `json:"value"`
		TextContent       string `json:"textContent"`
		ValueWrites       int    `json:"valueWrites"`
		TextContentWrites int    `json:"textContentWrites"`
		SelectionStart    int    `json:"selectionStart"`
		SelectionEnd      int    `json:"selectionEnd"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Value != "hello world" {
		t.Fatalf("textarea value = %q, want %q", got.Value, "hello world")
	}
	if got.TextContent != "default" {
		t.Fatalf("textarea textContent = %q, want untouched default", got.TextContent)
	}
	if got.ValueWrites != 1 {
		t.Fatalf("textarea value writes = %d, want 1", got.ValueWrites)
	}
	if got.TextContentWrites != 0 {
		t.Fatalf("textarea textContent writes = %d, want 0", got.TextContentWrites)
	}
	if got.SelectionStart != 11 || got.SelectionEnd != 11 {
		t.Fatalf("textarea selection = %d:%d, want 11:11", got.SelectionStart, got.SelectionEnd)
	}
}

func TestJawsJS_ValueKeepsSelectionInRangeWhenPrefixRemoved(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let value = "prefix body";
let selectionStart = 0;
let selectionEnd = 0;
let error = "";
const elem = {
	id: "Jid.1",
	tagName: "TEXTAREA",
	getAttribute: function() { return null; }
};
Object.defineProperty(elem, "value", {
	get: function() { return value; },
	set: function(v) { value = v; },
	enumerable: true,
	configurable: true,
});
Object.defineProperty(elem, "selectionStart", {
	get: function() { return selectionStart; },
	set: function(v) {
		if (v < 0 || v > value.length) {
			throw new Error("selectionStart out of range: " + v + " for " + value.length);
		}
		selectionStart = v;
	},
	enumerable: true,
	configurable: true,
});
Object.defineProperty(elem, "selectionEnd", {
	get: function() { return selectionEnd; },
	set: function(v) {
		if (v < 0 || v > value.length) {
			throw new Error("selectionEnd out of range: " + v + " for " + value.length);
		}
		selectionEnd = v;
	},
	enumerable: true,
	configurable: true,
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

try {
	jawsPerform("Value", "Jid.1", JSON.stringify("body"));
} catch (err) {
	error = String((err && err.message) || err);
}
process.stdout.write(JSON.stringify({
	value: value,
	selectionStart: selectionStart,
	selectionEnd: selectionEnd,
	error: error
}));
`)

	var got struct {
		Value          string `json:"value"`
		SelectionStart int    `json:"selectionStart"`
		SelectionEnd   int    `json:"selectionEnd"`
		Error          string `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Error != "" {
		t.Fatalf("Value update assigned an out-of-range textarea selection: %s", got.Error)
	}
	if got.Value != "body" {
		t.Fatalf("textarea value = %q, want %q", got.Value, "body")
	}
	if got.SelectionStart != 0 || got.SelectionEnd != 0 {
		t.Fatalf("textarea selection = %d:%d, want 0:0", got.SelectionStart, got.SelectionEnd)
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

func TestJawsJS_BatchedFrameIsolatesThrowingOrder(t *testing.T) {
	// The server coalesces independent element updates into one frame. A single
	// failing order (here a middle Inner targeting a missing element, which makes
	// jawsPerform throw "element not found") must not abandon the orders after it.
	raw := runJawsJSSnippet(t, `
jawsDebug = false;
var errors = [];
console.error = function(msg) { errors.push(msg); };

function makeElem(id) {
	var e = { id: id, _inner: "", querySelectorAll: function(){ return []; } };
	Object.defineProperty(e, "innerHTML", {
		get: function(){ return this._inner; },
		set: function(v){ this._inner = v; },
		enumerable: true, configurable: true,
	});
	return e;
}
var one = makeElem("Jid.1");
var two = makeElem("Jid.2");
document.getElementById = function(id) {
	if (id === "Jid.1") return one;
	if (id === "Jid.2") return two;
	return null; // Jid.9 is missing -> jawsPerform throws for that order only
};

var frame = [
	"Inner\tJid.1\t" + JSON.stringify("<b>one</b>"),
	"Inner\tJid.9\t" + JSON.stringify("<b>boom</b>"),
	"Inner\tJid.2\t" + JSON.stringify("<b>two</b>")
].join("\n");
jawsMessage({ data: frame });

process.stdout.write(JSON.stringify({ one: one.innerHTML, two: two.innerHTML, errorCount: errors.length }));
`)

	var got struct {
		One        string `json:"one"`
		Two        string `json:"two"`
		ErrorCount int    `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.One != "<b>one</b>" {
		t.Errorf("order before the throwing one was not applied: one=%q", got.One)
	}
	if got.Two != "<b>two</b>" {
		t.Errorf("order after the throwing one was dropped: two=%q", got.Two)
	}
	if got.ErrorCount != 1 {
		t.Errorf("expected exactly one console.error for the failing order, got %d", got.ErrorCount)
	}
}

func TestJawsJS_InsertNullPositionAppends(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let inserted = false;
const child = {
	id: "new-child",
	querySelectorAll: function() { return { forEach: function() {} }; }
};
const elem = {
	id: "Jid.1",
	children: [],
	insertBefore: function(node, where) {
		inserted = node === child && where === null;
	}
};
document.getElementById = function(id) {
	if (id === "Jid.1") return elem;
	return null;
};
document.createElement = function(tag) {
	if (tag !== "template") throw new Error("unexpected tag " + tag);
	const template = {};
	Object.defineProperty(template, "innerHTML", {
		set: function() { this.content = child; },
		enumerable: true,
		configurable: true,
	});
	return template;
};

jawsPerform("Insert", "Jid.1", JSON.stringify("null\n<span id=\"new-child\"></span>"));
process.stdout.write(JSON.stringify({ inserted: inserted }));
`)

	var got struct {
		Inserted bool `json:"inserted"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.Inserted {
		t.Fatal(`Insert with position "null" should append by calling insertBefore(node, null)`)
	}
}

func TestJawsJS_InsertNumericPositionIgnoresUnrelatedSameID(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let inserted = false;
let error = "";
const newChild = {
	id: "new-child",
	querySelectorAll: function() { return { forEach: function() {} }; }
};
const existingChild = { id: "existing-child" };
const unrelated = { id: "0", parentElement: null };
Object.setPrototypeOf(newChild, Node.prototype);
Object.setPrototypeOf(existingChild, Node.prototype);
Object.setPrototypeOf(unrelated, Node.prototype);
const elem = {
	id: "Jid.1",
	children: [existingChild],
	insertBefore: function(node, where) {
		if (where !== existingChild) {
			throw new Error("wrong reference " + ((where && where.id) || where));
		}
		inserted = node === newChild;
	}
};
existingChild.parentElement = elem;
document.getElementById = function(id) {
	if (id === "Jid.1") return elem;
	if (id === "0") return unrelated;
	return null;
};
document.createElement = function(tag) {
	if (tag !== "template") throw new Error("unexpected tag " + tag);
	const template = {};
	Object.defineProperty(template, "innerHTML", {
		set: function() { this.content = newChild; },
		enumerable: true,
		configurable: true,
	});
	return template;
};

try {
	jawsPerform("Insert", "Jid.1", JSON.stringify("0\n<span id=\"new-child\"></span>"));
} catch (err) {
	error = String((err && err.message) || err);
}
process.stdout.write(JSON.stringify({ inserted: inserted, error: error }));
`)

	var got struct {
		Inserted bool   `json:"inserted"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Error != "" {
		t.Fatalf("Insert with child index 0 used an unrelated id=\"0\" node: %s", got.Error)
	}
	if !got.Inserted {
		t.Fatal("Insert with child index 0 should insert before the first child")
	}
}

func TestJawsJS_OrderPreservesApplicationDataset(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const parent = { appended: [], appendChild: function(elem) { this.appended.push(elem.id); } };
const one = { id: "Jid.1", dataset: { jidsort: "application-one" }, parentElement: parent };
const two = { id: "Jid.2", dataset: { jidsort: "application-two" }, parentElement: parent };
document.getElementById = function(id) {
	if (id === "Jid.1") return one;
	if (id === "Jid.2") return two;
	return null;
};

jawsPerform("Order", "", JSON.stringify("Jid.2 Jid.1"));
process.stdout.write(JSON.stringify({
	appended: parent.appended,
	oneSort: one.dataset.jidsort || "",
	twoSort: two.dataset.jidsort || ""
}));
`)

	var got struct {
		Appended []string `json:"appended"`
		OneSort  string   `json:"oneSort"`
		TwoSort  string   `json:"twoSort"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if strings.Join(got.Appended, " ") != "Jid.2 Jid.1" {
		t.Fatalf("unexpected append order: %#v", got.Appended)
	}
	if got.OneSort != "application-one" || got.TwoSort != "application-two" {
		t.Fatalf("jawsOrder clobbered application data-jidsort: got %q and %q", got.OneSort, got.TwoSort)
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
