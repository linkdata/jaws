package assets

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/url"
	"os"
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
	const extraLogo = "logo.png"
	const extraUnknown = "data"
	const extraFont = "someExtraFont.woff2"
	const extraFontWithQuery = "someExtraFontQuery.woff2?x=1&copy=2"
	fontMime, _, fontMimeErr := mime.ParseMediaType(mime.TypeByExtension(".woff2"))
	if fontMimeErr != nil {
		fontMime = ""
	}

	serveJS, err := staticserve.New("/jaws/.jaws.js", []byte(JavascriptText))
	if err != nil {
		t.Fatal(err)
	}

	txt, fav := PreloadHTML()
	if txt != "" || fav != "" {
		t.Fatalf("PreloadHTML() = (%q, %q), want empty", txt, fav)
	}

	txt, fav = PreloadHTML(
		nil, // a nil URL argument is skipped
		mustParseURL(t, serveJS.Name),
		mustParseURL(t, extraScript),
		mustParseURL(t, extraScriptWithQuery),
		mustParseURL(t, extraStyle),
		mustParseURL(t, extraImage),
		mustParseURL(t, extraLogo),
		mustParseURL(t, extraUnknown),
		mustParseURL(t, extraFont),
		mustParseURL(t, extraFontWithQuery),
	)
	if !strings.Contains(txt, serveJS.Name) {
		t.Fatalf("missing %q in preload output: %q", serveJS.Name, txt)
	}
	if !strings.Contains(txt, extraScript) {
		t.Fatalf("missing %q in preload output: %q", extraScript, txt)
	}
	if !strings.Contains(txt, `<link rel="stylesheet" href="someExtraStyle.css">`) {
		t.Fatalf("stylesheet destination missing for %q in preload output: %q", extraStyle, txt)
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
	// Common font extensions have a deterministic font destination even when the
	// local MIME database does not provide a type attribute.
	wantFontLink := `<link rel="preload" href="someExtraFont.woff2" as="font"`
	if strings.HasPrefix(fontMime, "font/") {
		wantFontLink += ` type="` + fontMime + `"`
	}
	wantFontLink += ` crossorigin="anonymous">`
	if !strings.Contains(txt, wantFontLink) {
		t.Fatalf("missing structured font preload %q in %q", wantFontLink, txt)
	}

	pngMime, _, err := mime.ParseMediaType(mime.TypeByExtension(".png"))
	if err != nil {
		t.Fatal(err)
	}
	wantFaviconLink := `<link rel="icon" type="` + pngMime + `" href="favicon.png">`
	if !strings.Contains(txt, wantFaviconLink) {
		t.Fatalf("missing structured favicon link %q in %q", wantFaviconLink, txt)
	}

	// A non-favicon image is emitted as an ordinary preload link carrying both
	// as="image" and the resolved image/png type, mirroring the font assertion.
	wantLogoLink := `<link rel="preload" href="logo.png" as="image" type="` + pngMime + `">`
	if !strings.Contains(txt, wantLogoLink) {
		t.Fatalf("missing structured image preload %q in %q", wantLogoLink, txt)
	}

	if strings.Contains(txt, `href="`+extraUnknown+`"`) {
		t.Fatalf("unclassified resource appears in preload output: %q", txt)
	}

	if fav != extraImage {
		t.Fatalf("favicon = %q, want %q", fav, extraImage)
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func Test_PreloadHTML_VersionedCommonResources(t *testing.T) {
	htmlCode, faviconURL := PreloadHTML(
		&url.URL{Path: "app.js@4.4.1"},
		&url.URL{Path: "theme.css@latest"},
		&url.URL{Path: "logo.png@1.json"},
		&url.URL{Path: "favicon.png@2"},
		&url.URL{Path: "face.woff2@v3"},
	)
	want := `<link rel="preload" href="logo.png@1.json" as="image">
<link rel="preload" href="face.woff2@v3" as="font" crossorigin="anonymous">
<link rel="stylesheet" href="theme.css@latest">
<link rel="icon" href="favicon.png@2">
<script defer src="app.js@4.4.1"></script>
`
	if htmlCode != want {
		t.Fatalf("PreloadHTML() = %q, want %q", htmlCode, want)
	}
	if faviconURL != "favicon.png@2" {
		t.Fatalf("faviconURL = %q, want %q", faviconURL, "favicon.png@2")
	}
}

func Test_PreloadHTML_OmitsUncommonResources(t *testing.T) {
	const (
		scriptExt = ".jawspreloadscript"
		styleExt  = ".jawspreloadstyle"
		wasmExt   = ".jawspreloadwasm"
	)
	for ext, typ := range map[string]string{
		scriptExt: "TEXT/JAVASCRIPT",
		styleExt:  "TEXT/CSS",
		wasmExt:   "application/wasm",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			t.Fatalf("AddExtensionType(%q, %q): %v", ext, typ, err)
		}
	}

	htmlCode, faviconURL := PreloadHTML(
		&url.URL{Path: "module.mjs"},
		&url.URL{Path: "script" + scriptExt},
		&url.URL{Path: "style" + styleExt},
		&url.URL{Path: "app.js@1" + scriptExt},
		&url.URL{Path: "theme.css@1" + styleExt},
		&url.URL{Path: "module" + wasmExt},
		&url.URL{Scheme: "wss", Host: "events.example.test", Path: "/socket.js"},
	)
	if htmlCode != "" {
		t.Fatalf("PreloadHTML() = %q, want empty", htmlCode)
	}
	if faviconURL != "" {
		t.Fatalf("faviconURL = %q, want empty", faviconURL)
	}
}

// Test_PreloadHTML_MultipleFaviconsLastWins pins the documented contract that when
// several resources qualify as favicons, only the last is honored (returned as
// faviconURL and emitted as the rel="icon" link) and the earlier ones are discarded
// entirely rather than emitted as preload links.
func Test_PreloadHTML_MultipleFaviconsLastWins(t *testing.T) {
	txt, fav := PreloadHTML(
		mustParseURL(t, "favicon.png"),
		mustParseURL(t, "favicon-dark.png"),
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

// Test_PreloadHTML_MIMEFamilyMatching pins that MIME families are matched on the
// "type/" prefix case-insensitively. False prefixes such as "imagery/*" and
// "fontastic/*" must not be mistaken for "image/*" or "font/*", while differently
// cased valid families such as "IMAGE/*" and "FONT/*" must still be recognized.
func Test_PreloadHTML_MIMEFamilyMatching(t *testing.T) {
	// Distinct ".jaws*" extensions keep these process-global registrations from
	// colliding with real MIME mappings or with other tests in this package.
	reg := map[string]string{
		".jawsimagery":       "imagery/not-an-image",
		".jawsfontastic":     "fontastic/not-a-font",
		".jawsupperimg":      "IMAGE/x-icon",
		".jawsupperfont":     "FONT/woff2",
		".js@jawsimage":      "IMAGE/x-versioned",
		".css@jawsversioned": "FONT/x-versioned",
	}
	for ext, typ := range reg {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			t.Fatalf("AddExtensionType(%q, %q): %v", ext, typ, err)
		}
	}

	txt, fav := PreloadHTML(
		mustParseURL(t, "favicon.jawsimagery"),  // imagery/* is not image/*
		mustParseURL(t, "brand.jawsfontastic"),  // fontastic/* is not font/*
		mustParseURL(t, "favicon.jawsupperimg"), // IMAGE/* is image/* (case-insensitive)
		mustParseURL(t, "brand.jawsupperfont"),  // FONT/* is font/* (case-insensitive)
		mustParseURL(t, "logo.js@jawsimage"),
		mustParseURL(t, "face.css@jawsversioned"),
	)

	// False MIME-family prefixes are not common resources and are omitted.
	if strings.Contains(txt, "favicon.jawsimagery") {
		t.Errorf("imagery/* resource appears in %q", txt)
	}
	if strings.Contains(txt, "brand.jawsfontastic") {
		t.Errorf("fontastic/* resource appears in %q", txt)
	}

	// IMAGE/* qualifies as an image, so the favicon-named resource wins the favicon
	// slot and is emitted as the rel="icon" link.
	if fav != "favicon.jawsupperimg" {
		t.Errorf("favicon = %q, want %q", fav, "favicon.jawsupperimg")
	}
	wantFaviconLink := `<link rel="icon" type="image/x-icon" href="favicon.jawsupperimg">`
	if !strings.Contains(txt, wantFaviconLink) {
		t.Errorf("case-insensitive favicon = missing %q in %q", wantFaviconLink, txt)
	}

	// FONT/* qualifies as a font, so it receives as="font" and anonymous CORS.
	wantUpperFont := `<link rel="preload" href="brand.jawsupperfont" as="font" type="font/woff2" crossorigin="anonymous">`
	if !strings.Contains(txt, wantUpperFont) {
		t.Errorf("case-insensitive font preload = missing %q in %q", wantUpperFont, txt)
	}
	if want := `<link rel="preload" href="logo.js@jawsimage" as="image" type="image/x-versioned">`; !strings.Contains(txt, want) {
		t.Errorf("final image MIME = missing %q in %q", want, txt)
	}
	if want := `<link rel="preload" href="face.css@jawsversioned" as="font" type="font/x-versioned" crossorigin="anonymous">`; !strings.Contains(txt, want) {
		t.Errorf("final font MIME = missing %q in %q", want, txt)
	}
}

func runJawsJSSnippet(t *testing.T, snippet string) string {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		// The node-driven JS behavior tests are the strongest guarantees in this
		// package, so a host without node must not let them silently no-op. When
		// JAWS_REQUIRE_NODE is set (CI does) a missing node fails loudly; otherwise
		// it is a skip so a local "go test" on a node-less machine still passes.
		if os.Getenv("JAWS_REQUIRE_NODE") != "" {
			t.Fatal("node executable not available but JAWS_REQUIRE_NODE is set")
		}
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
const windowListeners = {};

global.window = {
	location: { protocol: "http:", host: "example.test", reload: function(){}, assign: function(){} },
	addEventListener: function(name, fn){
		(windowListeners[name] ||= []).push(fn);
	},
	removeEventListener: function(name, fn){
		windowListeners[name] = (windowListeners[name] || []).filter(function(other) {
			return other !== fn;
		});
	},
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
global.jawsDispatchWindowEvent = function(name, event) {
	if (!event) event = { type: name };
	(windowListeners[name] || []).slice().forEach(function(fn) { fn(event); });
};

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

func TestJawsJS_CanonicalJidPredicate(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const values = [
	"Jid.1",
	"Jid.9",
	"Jid.10",
	"Jid.9223372036854775807",
	"",
	"id",
	"Jid.",
	"Jid.0",
	"Jid.00",
	"Jid.01",
	"Jid.-1",
	"Jid.+1",
	"Jid.1x",
	"Jid.9223372036854775808",
	"Jid.999999999999999999999999",
	1,
	null
];
process.stdout.write(JSON.stringify(values.map(jawsIsJid)));
`)

	var got []bool
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	want := []bool{true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("jawsIsJid results = %v, want %v", got, want)
	}
}

func TestJawsJS_AttachRejectsNoncanonicalJids(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function makeElem(id) {
	return {
		id: id,
		tagName: "DIV",
		listeners: [],
		hasAttribute: function() { return false; },
		addEventListener: function(name) { this.listeners.push(name); }
	};
}

const valid = makeElem("Jid.1");
const zero = makeElem("Jid.0");
const leadingZero = makeElem("Jid.01");
const arbitrary = makeElem("application-id");
jawsAttach(valid);
jawsAttach(zero);
jawsAttach(leadingZero);
jawsAttach(arbitrary);
process.stdout.write(JSON.stringify({
	valid: valid.listeners,
	zero: zero.listeners,
	leadingZero: leadingZero.listeners,
	arbitrary: arbitrary.listeners
}));
`)

	var got struct {
		Valid       []string `json:"valid"`
		Zero        []string `json:"zero"`
		LeadingZero []string `json:"leadingZero"`
		Arbitrary   []string `json:"arbitrary"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Valid, []string{"click", "contextmenu"}) {
		t.Fatalf("canonical Jid listeners = %v", got.Valid)
	}
	if len(got.Zero) != 0 || len(got.LeadingZero) != 0 || len(got.Arbitrary) != 0 {
		t.Fatalf("noncanonical elements were attached: %+v", got)
	}
}

func TestJawsJS_NumberUsesChangeBeforeAutoSubmit(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) {
	this.sent.push(msg);
	log.push("send");
};
WebSocket = FakeSocket;
jaws = new FakeSocket();

const log = [];
let stopped = false;
const listeners = {};
const number = {
	id: "Jid.1",
	tagName: "INPUT",
	type: "number",
	value: "1",
	form: { submit: function() { log.push("submit"); } },
	hasAttribute: function(name) {
		return name === "data-jawsnumber" || name === "data-jawsonchangesubmit";
	},
	getAttribute: function(name) { return name === "type" ? "number" : null; },
	addEventListener: function(name, fn) { (listeners[name] ||= []).push(fn); }
};
const rangeListeners = {};
const range = {
	id: "Jid.2",
	tagName: "INPUT",
	type: "range",
	value: "1",
	hasAttribute: function() { return false; },
	getAttribute: function(name) { return name === "type" ? "range" : null; },
	addEventListener: function(name, fn) { (rangeListeners[name] ||= []).push(fn); }
};
const top = {
	querySelectorAll: function(selector) {
		if (selector === '[id^="' + jawsIdPrefix + '"]') {
			return [number, range];
		}
		if (selector === '[data-jawsonchangesubmit]') {
			return [number];
		}
		return [];
	}
};

jawsAttachChildren(top);
function dispatchChange() {
	const ev = new Event();
	ev.currentTarget = number;
	ev.stopPropagation = function() { stopped = true; };
	(listeners.change || []).forEach(function(fn) { fn.call(number, ev); });
}

number.value = "9007199254740993";
dispatchChange();

process.stdout.write(JSON.stringify({
	numberInputListeners: (listeners.input || []).length,
	numberChangeListeners: (listeners.change || []).length,
	rangeInputListeners: (rangeListeners.input || []).length,
	rangeChangeListeners: (rangeListeners.change || []).length,
	frames: jaws.sent,
	log: log,
	stopped: stopped
}));
`)

	var got struct {
		NumberInputListeners  int      `json:"numberInputListeners"`
		NumberChangeListeners int      `json:"numberChangeListeners"`
		RangeInputListeners   int      `json:"rangeInputListeners"`
		RangeChangeListeners  int      `json:"rangeChangeListeners"`
		Frames                []string `json:"frames"`
		Log                   []string `json:"log"`
		Stopped               bool     `json:"stopped"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.NumberInputListeners != 0 || got.NumberChangeListeners != 2 {
		t.Fatalf("Number listeners = input:%d change:%d, want input:0 change:2", got.NumberInputListeners, got.NumberChangeListeners)
	}
	if got.RangeInputListeners != 1 || got.RangeChangeListeners != 0 {
		t.Fatalf("Range listeners = input:%d change:%d, want input:1 change:0", got.RangeInputListeners, got.RangeChangeListeners)
	}
	if !reflect.DeepEqual(got.Log, []string{"send", "submit"}) {
		t.Fatalf("Number change order = %v, want send before submit", got.Log)
	}
	if !got.Stopped {
		t.Fatal("Number change handler did not stop propagation")
	}
	if len(got.Frames) != 1 {
		t.Fatalf("Number frames = %q, want one frame", got.Frames)
	}
	msg, ok := wire.Parse([]byte(got.Frames[0]))
	if !ok || msg.What != what.Input || msg.Jid != 1 || msg.Data != "9007199254740993" {
		t.Fatalf("unexpected Number frame: %+v, parseable %t", msg, ok)
	}
}

func TestJawsJS_ClickAndInputRoutesRejectNoncanonicalJids(t *testing.T) {
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
	id: "Jid.02",
	tagName: "DIV",
	getAttribute: function() { return null; },
	textContent: "",
	parentElement: parent
};
const input = {
	id: "application-input",
	tagName: "INPUT",
	getAttribute: function() { return "text"; },
	value: "typed"
};
const inputEvent = new Event();
inputEvent.currentTarget = input;
let stopped = false;
inputEvent.stopPropagation = function() { stopped = true; };
jawsInputHandler(inputEvent);

const clickEvent = new Event();
clickEvent.clientX = 1;
clickEvent.clientY = 2;
clickEvent.shiftKey = false;
clickEvent.ctrlKey = false;
clickEvent.altKey = false;
process.stdout.write(JSON.stringify({
	clickData: jawsBuildClickData(target, clickEvent),
	inputFrames: jaws.sent,
	inputStopped: stopped
}));
`)

	var got struct {
		ClickData    string   `json:"clickData"`
		InputFrames  []string `json:"inputFrames"`
		InputStopped bool     `json:"inputStopped"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.ClickData != "1 2 0 Jid.02\tJid.1" {
		t.Fatalf("click data = %q, want only the canonical ancestor route", got.ClickData)
	}
	if len(got.InputFrames) != 0 || got.InputStopped {
		t.Fatalf("noncanonical input route was handled: %+v", got)
	}
}

func TestJawsJS_ConnectsAfterDeferredAssets(t *testing.T) {
	for _, readyEvent := range []string{"DOMContentLoaded", "load"} {
		t.Run(readyEvent, func(t *testing.T) {
			// The ready event is the "deferred assets loaded" signal: a page's
			// deferred scripts run before DOMContentLoaded, so connecting on the
			// ready event connects after them. Assert jaws.js connects exactly once,
			// only after the event, and stays idempotent across duplicate events.
			raw := runJawsJSSnippet(t, `
let socketCount = 0;
function FakeSocket() {
	this.readyState = 1;
	socketCount++;
}

FakeSocket.prototype.addEventListener = function() {};
WebSocket = FakeSocket;

const before = socketCount;
jawsDispatchWindowEvent("`+readyEvent+`");
jawsDispatchWindowEvent("DOMContentLoaded");
jawsDispatchWindowEvent("load");

process.stdout.write(JSON.stringify({
	before: before,
	after: socketCount
}));
`)

			var got struct {
				Before int `json:"before"`
				After  int `json:"after"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
				t.Fatalf("failed to parse snippet output %q: %v", raw, err)
			}
			if got.Before != 0 || got.After != 1 {
				t.Fatalf("connection state = %+v, want one connection after the ready event", got)
			}
		})
	}
}

func TestJawsJS_StoreReadAndOptimisticProposal(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();
const elem = {
	id: "Jid.9",
	dataset: { jawsstore: "app", jawsdata: '{"state":{"value":1},"items":[10,20]}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
document.querySelector = function(selector) { return selector === '[data-jawsstore="app"]' ? elem : null; };
jawsAttach(elem, true);

const initial = jawsVar("app.state.value");
const readFrames = jaws.sent.length;
const accepted = jawsVar("app.state.value", 42);
const afterWrite = jawsVar("app.state.value");
const invalidIndex = jawsVar("app.items.01", 99);
let invalidIntermediate = false;
try { jawsVar("app.items.01.value", 99); }
catch { invalidIntermediate = true; }
const sideProperty = Object.hasOwn(jawsVar("app.items"), "01");
const arrayWrite = jawsVar("app.items.1", 21);
const frames = jaws.sent.slice();

jaws.send = function() { throw new Error("send failed"); };
const sendFailed = jawsVar("app.state.value", 100);
const serializationFailed = jawsVar("app.state.value", 1n);
jaws.readyState = 0;
const closed = jawsVar("app.state.value", 200);
const unboundWrite = jawsVar("absent", 1);
process.stdout.write(JSON.stringify({
	initial: initial,
	readFrames: readFrames,
	accepted: accepted,
	afterWrite: afterWrite,
	invalidIndex: invalidIndex,
	invalidIntermediate: invalidIntermediate,
	sideProperty: sideProperty,
	arrayWrite: arrayWrite,
	arrayValue: jawsVar("app.items.1"),
	frames: frames,
	sendFailed: sendFailed,
	serializationFailed: serializationFailed,
	closed: closed,
	unboundWrite: unboundWrite,
	unboundRead: jawsVar("absent"),
	missingRead: jawsVar("stillAbsent") === undefined,
	finalValue: jawsVar("app.state.value"),
}));
`)
	var got struct {
		Initial             int      `json:"initial"`
		ReadFrames          int      `json:"readFrames"`
		Accepted            bool     `json:"accepted"`
		AfterWrite          int      `json:"afterWrite"`
		InvalidIndex        bool     `json:"invalidIndex"`
		InvalidIntermediate bool     `json:"invalidIntermediate"`
		SideProperty        bool     `json:"sideProperty"`
		ArrayWrite          bool     `json:"arrayWrite"`
		ArrayValue          int      `json:"arrayValue"`
		Frames              []string `json:"frames"`
		SendFailed          bool     `json:"sendFailed"`
		SerializationFailed bool     `json:"serializationFailed"`
		Closed              bool     `json:"closed"`
		UnboundWrite        bool     `json:"unboundWrite"`
		UnboundRead         int      `json:"unboundRead"`
		MissingRead         bool     `json:"missingRead"`
		FinalValue          int      `json:"finalValue"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("store output %q: %v", raw, err)
	}
	if got.Initial != 1 || got.ReadFrames != 1 || !got.Accepted || got.AfterWrite != 42 ||
		got.InvalidIndex || !got.InvalidIntermediate || got.SideProperty || !got.ArrayWrite || got.ArrayValue != 21 ||
		got.SendFailed || got.SerializationFailed || got.Closed || !got.UnboundWrite || got.UnboundRead != 1 ||
		!got.MissingRead || got.FinalValue != 42 {
		t.Fatalf("store read/write state = %+v", got)
	}
	if len(got.Frames) != 5 {
		t.Fatalf("proposal frames = %q, want five", got.Frames)
	}
	for i, want := range []string{"state.value=1", "state.value=42", "state.value=42", "items=[10,20]", "items.1=21"} {
		msg, ok := wire.Parse([]byte(got.Frames[i]))
		if !ok || msg.What != what.JsVar || msg.Jid != 9 || msg.Data != want {
			t.Fatalf("proposal %d = %+v, parseable %t; want Jid.9 %q", i, msg, ok, want)
		}
	}
}

func TestJawsJS_PatchAndRemovedBinding(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();
const elem = {
	id: "Jid.9",
	dataset: { jawsstore: "app", jawsdata: '{"state":1,"obsolete":5,"items":[1,2]}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
let attached = true;
document.getElementById = function(id) { return attached && id === elem.id ? elem : null; };
document.querySelector = function(selector) { return attached && selector === '[data-jawsstore="app"]' ? elem : null; };
jawsAttach(elem, true);
jawsPerform("JsVar", "Jid.9", "items.1=3");
jawsPerform("JsVar", "Jid.9", "obsolete=");
const deleted = !Object.hasOwn(jawsVar("app"), "obsolete");
jawsPerform("JsVar", "Jid.9", '={"state":7}');
const state = jawsVar("app");
attached = false;
jawsPerform("JsVar", "Jid.9", "state=8");
const afterRemoval = jawsVar("app");
const unboundWrite = jawsVar("app", {state: 9});
process.stdout.write(JSON.stringify({
	deleted: deleted,
	state: state,
	afterRemoval: afterRemoval,
	unboundWrite: unboundWrite,
	afterWrite: window.app,
	frames: jaws.sent,
	windowGlobal: Object.hasOwn(window, "app"),
}));
`)
	var got struct {
		Deleted      bool           `json:"deleted"`
		State        map[string]int `json:"state"`
		AfterRemoval map[string]int `json:"afterRemoval"`
		UnboundWrite bool           `json:"unboundWrite"`
		AfterWrite   map[string]int `json:"afterWrite"`
		Frames       []string       `json:"frames"`
		WindowGlobal bool           `json:"windowGlobal"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("patch output %q: %v", raw, err)
	}
	if !got.Deleted || !reflect.DeepEqual(got.State, map[string]int{"state": 7}) ||
		!reflect.DeepEqual(got.AfterRemoval, map[string]int{"state": 7}) ||
		!got.UnboundWrite || !reflect.DeepEqual(got.AfterWrite, map[string]int{"state": 9}) ||
		len(got.Frames) != 2 || !got.WindowGlobal {
		t.Fatalf("patch state = %+v", got)
	}
}

func TestJawsJS_LiveWindowPaths(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();
window.vendor = {chart: {count: 0}};
window.vendorFunc = function() {};
window.vendorFunc.state = {count: 0};
window.library = Object.create({version: 1});
window.library['odd"name'] = 7;
window.location.href = "https://example.test/";
const elem = {
	id: "Jid.12",
	dataset: { jawsstore: "vendor.chart", jawsdata: '{"count":1}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
const functionElem = {
	id: "Jid.13",
	dataset: { jawsstore: "vendorFunc.state", jawsdata: '{"count":1}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
let escapedSelector = false;
document.querySelector = function(selector) {
	if (selector.includes('odd\\"name')) escapedSelector = true;
	if (selector === '[data-jawsstore="vendor.chart"]') return elem;
	if (selector === '[data-jawsstore="vendorFunc.state"]') return functionElem;
	return null;
};
document.getElementById = function(id) {
	if (id === elem.id) return elem;
	if (id === functionElem.id) return functionElem;
	return null;
};
jawsAttach(elem, true);
jawsAttach(functionElem, true);
jawsPerform("JsVar", "Jid.13", '={"count":5}');
const initial = window.vendor.chart.count;
const wrote = jawsVar("vendor.chart.count", 2);
jawsPerform("JsVar", "Jid.12", "count=3");
const patched = window.vendor.chart;
jawsPerform("JsVar", "Jid.12", '={"count":3}');
const retained = patched === window.vendor.chart;
const read = jawsVar("vendor.chart.count");
window.vendor.chart.count = 4;
const direct = jawsVar("vendor.chart").count;
const href = jawsVar("location.href");
const hrefWrote = jawsVar("location.href", "https://next.test/");
const inherited = jawsVar("library.version");
const odd = jawsVar('library.odd"name');
const libraryWrote = jawsVar("library.version", 2);
process.stdout.write(JSON.stringify({
	initial: initial,
	wrote: wrote,
	retained: retained,
	read: read,
	direct: direct,
	href: href,
	hrefWrote: hrefWrote,
	newHref: window.location.href,
	inherited: inherited,
	odd: odd,
	escapedSelector: escapedSelector,
	libraryWrote: libraryWrote,
	newVersion: window.library.version,
	functionState: window.vendorFunc.state.count,
	frames: jaws.sent,
}));
`)
	var got struct {
		Initial         int      `json:"initial"`
		Wrote           bool     `json:"wrote"`
		Retained        bool     `json:"retained"`
		Read            int      `json:"read"`
		Direct          int      `json:"direct"`
		Href            string   `json:"href"`
		HrefWrote       bool     `json:"hrefWrote"`
		NewHref         string   `json:"newHref"`
		Inherited       int      `json:"inherited"`
		Odd             int      `json:"odd"`
		EscapedSelector bool     `json:"escapedSelector"`
		LibraryWrote    bool     `json:"libraryWrote"`
		NewVersion      int      `json:"newVersion"`
		FunctionState   int      `json:"functionState"`
		Frames          []string `json:"frames"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("live paths output %q: %v", raw, err)
	}
	if got.Initial != 1 || !got.Wrote || !got.Retained || got.Read != 3 || got.Direct != 4 ||
		got.Href != "https://example.test/" || !got.HrefWrote || got.NewHref != "https://next.test/" ||
		got.Inherited != 1 || got.Odd != 7 || !got.EscapedSelector ||
		!got.LibraryWrote || got.NewVersion != 2 ||
		got.FunctionState != 5 || len(got.Frames) != 3 {
		t.Fatalf("live paths = %+v", got)
	}
	for i, want := range []string{"count=2", "count=3", `={"count":4}`} {
		msg, ok := wire.Parse([]byte(got.Frames[i]))
		if !ok || msg.What != what.JsVar || msg.Jid != 12 || msg.Data != want {
			t.Fatalf("proposal %d = %+v, parseable %t; want Jid.12 %q", i, msg, ok, want)
		}
	}
}

func TestJawsJS_BindingRequiresInitialAttach(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const elem = {
	id: "Jid.9",
	dataset: { jawsstore: "app", jawsdata: '{"value":1}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
const fragment = {
	querySelectorAll: function(selector) {
		return selector === '[id^="' + jawsIdPrefix + '"]' ? [elem] : [];
	},
};
let rejected = false;
try { jawsAttachChildren(fragment); } catch { rejected = true; }
process.stdout.write(JSON.stringify({ rejected: rejected, initialized: Object.hasOwn(window, "app") }));
`)
	var got struct {
		Rejected    bool `json:"rejected"`
		Initialized bool `json:"initialized"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("binding output %q: %v", raw, err)
	}
	if !got.Rejected || got.Initialized {
		t.Fatalf("late binding = %+v", got)
	}
}

func TestJawsJS_PatchFailureReloads(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let reloads = 0;
let errors = 0;
window.location.reload = function() { reloads++; };
console.error = function() { errors++; };
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
FakeSocket.prototype.removeEventListener = function() {};
FakeSocket.prototype.close = function() {};
WebSocket = FakeSocket;
jaws = new FakeSocket();
const elem = {
	id: "Jid.9",
	dataset: { jawsstore: "app", jawsdata: '{"state":{"value":1}}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
document.getElementById = function(id) { return id === elem.id ? elem : null; };
document.querySelector = function(selector) { return selector === '[data-jawsstore="app"]' ? elem : null; };
jawsAttach(elem, true);
jawsVar("app.state", 0);
jawsMessage({data: "JsVar\tJid.9\tstate.value=2\nJsVar\tJid.9\tstate=9\n"});
process.stdout.write(JSON.stringify({reloads: reloads, errors: errors, state: jawsVar("app.state")}));
`)
	var got struct {
		Reloads int `json:"reloads"`
		Errors  int `json:"errors"`
		State   int `json:"state"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("patch failure output %q: %v", raw, err)
	}
	if got.Reloads != 1 || got.Errors != 0 || got.State != 0 {
		t.Fatalf("failed patch handling = %+v", got)
	}
}

func TestJawsJS_StorePathGuards(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();
const elem = {
	id: "Jid.9",
	dataset: { jawsstore: "app", jawsdata: '{}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
document.querySelector = function(selector) { return selector === '[data-jawsstore="app"]' ? elem : null; };
jawsAttach(elem, true);
const originalPrototype = Object.getPrototypeOf(jawsVar("app"));
const forbidden = ["app.__proto__", "app.constructor", "app.prototype", "app..state", "app.state=bad", "app.state\tbad", "app.state\x1bbad", "app.state\x7fbad"];
const rejected = forbidden.map(function(path) {
	try { jawsVar(path, {polluted: true}); return false; }
	catch { return true; }
});
const nearName = jawsVar("app.__proto", 5);
process.stdout.write(JSON.stringify({
	rejected: rejected,
	nearName: nearName,
	nearValue: jawsVar("app.__proto"),
	frames: jaws.sent.length,
	prototypeUnchanged: Object.getPrototypeOf(jawsVar("app")) === originalPrototype,
	objectPolluted: Object.hasOwn(Object.prototype, "polluted"),
}));
`)
	var got struct {
		Rejected           []bool `json:"rejected"`
		NearName           bool   `json:"nearName"`
		NearValue          int    `json:"nearValue"`
		Frames             int    `json:"frames"`
		PrototypeUnchanged bool   `json:"prototypeUnchanged"`
		ObjectPolluted     bool   `json:"objectPolluted"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("guard output %q: %v", raw, err)
	}
	for i, rejected := range got.Rejected {
		if !rejected {
			t.Errorf("unsafe path %d was accepted", i)
		}
	}
	if len(got.Rejected) != 8 || !got.NearName ||
		got.NearValue != 5 || got.Frames != 3 || !got.PrototypeUnchanged || got.ObjectPolluted {
		t.Fatalf("store path guards = %+v", got)
	}
}

func TestJawsJS_RemoveFromNonManagedContainerIsRejected(t *testing.T) {
	raw := runJawsJSSnippet(t, `
	function FakeSocket() { this.readyState = 1; this.sent = []; }
	FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const topElem = {
	id: "container",
	querySelectorAll: function() {
		throw new Error("non-managed container was queried");
	}
};
jawsRemoving(topElem);
process.stdout.write(JSON.stringify(jaws.sent));
`)

	if raw != "[]" {
		t.Fatalf("jawsRemoving sent a frame for a non-managed container: %q", raw)
	}
}

func TestJawsJS_RemovingReportsOnlyCanonicalDescendantJids(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const topElem = {
	id: "Jid.1",
	querySelectorAll: function() {
		return [
			{ id: "Jid.2" },
			{ id: "Jid.03" },
			{ id: "application-id" },
			{ id: "Jid.0" },
			{ id: "Jid.4" }
		];
	}
};
jawsRemoving(topElem);
process.stdout.write(jaws.sent[0] || "");
`)

	msg, ok := wire.Parse([]byte(raw))
	if !ok {
		t.Fatalf("Remove frame must be parseable by wire.Parse, got %q", raw)
	}
	if msg.What != what.Remove || msg.Jid != 1 || msg.Data != "Jid.2\tJid.4" {
		t.Fatalf("unexpected removal frame: %+v", msg)
	}
}

func TestJawsJS_RequestScopedCallDoesNotRequireElement(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let called = null;
let lookedUp = false;
window.app = Object.create({
	refresh: function(value) { called = value; }
});
document.getElementById = function() {
	lookedUp = true;
	return null;
};

let thrown = "";
try {
	jawsPerform("Call", "", 'app.refresh={"source":"server"}');
} catch (err) {
	thrown = String(err);
}
process.stdout.write(JSON.stringify({ called: called, lookedUp: lookedUp, thrown: thrown }));
`)

	var got struct {
		Called struct {
			Source string `json:"source"`
		} `json:"called"`
		LookedUp bool   `json:"lookedUp"`
		Thrown   string `json:"thrown"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if got.Thrown != "" {
		t.Fatalf("request-scoped Call failed before invocation: %s", got.Thrown)
	}
	if got.LookedUp {
		t.Fatal("request-scoped Call attempted an element lookup")
	}
	if got.Called.Source != "server" {
		t.Fatalf("request-scoped Call argument = %+v, want source=server", got.Called)
	}
}

func TestJawsJS_IsCommandRoutesElementBoundPatch(t *testing.T) {
	patch := wire.WsMsg{What: what.JsVar, Data: "state=2"}
	if !patch.What.IsCommand() {
		patch.Jid = 1
	}
	call := wire.WsMsg{What: what.Call, Data: "app.notify=3"}
	if !call.What.IsCommand() {
		call.Jid = 1
	}
	frame, err := json.Marshal(patch.Format() + call.Format())
	if err != nil {
		t.Fatal(err)
	}

	raw := runJawsJSSnippet(t, `
let notified = 0;
let errors = [];
let lookups = [];
window.app = { notify: function(value) { notified = value; } };
const elem = {
	id: "Jid.1",
	dataset: { jawsstore: "store", jawsdata: '{"state":0}' },
	hasAttribute: function(attr) { return attr === "data-jawsstore"; },
};
jawsAttach(elem, true);
document.getElementById = function(id) {
	lookups.push(id);
	return id === elem.id ? elem : null;
};
console.error = function(message) { errors.push(String(message)); };

jawsMessage({ data: `+string(frame)+` });

process.stdout.write(JSON.stringify({
	state: jawsVar("store.state"),
	notified: notified,
	errors: errors,
	lookups: lookups,
}));
`)

	var got struct {
		State    int      `json:"state"`
		Notified int      `json:"notified"`
		Errors   []string `json:"errors"`
		Lookups  []string `json:"lookups"`
	}
	if err = json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if got.State != 2 {
		t.Errorf("Patch value = %d, want 2", got.State)
	}
	if got.Notified != 3 {
		t.Errorf("Call argument = %d, want 3", got.Notified)
	}
	if len(got.Errors) != 0 {
		t.Errorf("client errors = %q, want none", got.Errors)
	}
	if want := []string{"Jid.1"}; !reflect.DeepEqual(got.Lookups, want) {
		t.Errorf("element lookups = %q, want %q", got.Lookups, want)
	}
}

func TestJawsJS_ElementScopedCallStillRequiresElement(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let calls = 0;
let lookups = [];
window.app = {
	refresh: function() { calls++; }
};
document.getElementById = function(id) {
	lookups.push(id);
	return id === "Jid.1" ? { id: id } : null;
};

jawsPerform("Call", "Jid.1", 'app.refresh={}');
let missingError = "";
try {
	jawsPerform("Call", "Jid.2", 'app.refresh={}');
} catch (err) {
	missingError = String(err);
}
process.stdout.write(JSON.stringify({ calls: calls, lookups: lookups, missingError: missingError }));
`)

	var got struct {
		Calls        int      `json:"calls"`
		Lookups      []string `json:"lookups"`
		MissingError string   `json:"missingError"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unexpected JSON output %q: %v", raw, err)
	}
	if got.Calls != 1 {
		t.Fatalf("element-scoped calls = %d, want 1", got.Calls)
	}
	if !reflect.DeepEqual(got.Lookups, []string{"Jid.1", "Jid.2"}) {
		t.Fatalf("element lookups = %#v, want both targeted IDs", got.Lookups)
	}
	if !strings.Contains(got.MissingError, "element not found: Jid.2") {
		t.Fatalf("missing element error = %q", got.MissingError)
	}
}

func TestJawsJS_PerformRejectsNoncanonicalTargetsBeforeLookup(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const lookups = [];
const elem = {
	id: "Jid.1",
	innerHTML: "",
	querySelectorAll: function() { return []; }
};
document.getElementById = function(id) {
	lookups.push(id);
	return id === "Jid.1" ? elem : null;
};

const ids = ["application-id", "Jid.0", "Jid.01", "Jid.-1", ""];
const errors = [];
ids.forEach(function(id) {
	try {
		jawsPerform("Inner", id, JSON.stringify("bad"));
	} catch (err) {
		errors.push(String(err));
	}
});
jawsPerform("Inner", "Jid.1", JSON.stringify("good"));
process.stdout.write(JSON.stringify({ lookups: lookups, errors: errors, html: elem.innerHTML }));
`)

	var got struct {
		Lookups []string `json:"lookups"`
		Errors  []string `json:"errors"`
		HTML    string   `json:"html"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Lookups, []string{"Jid.1"}) {
		t.Fatalf("element lookups = %v, want only the canonical Jid", got.Lookups)
	}
	if len(got.Errors) != 5 {
		t.Fatalf("noncanonical target errors = %v", got.Errors)
	}
	for _, errstr := range got.Errors {
		if !strings.Contains(errstr, "invalid Jid") {
			t.Fatalf("unexpected target error %q", errstr)
		}
	}
	if got.HTML != "good" {
		t.Fatalf("canonical target update = %q", got.HTML)
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

func TestJawsJS_PerformValuePreservesImplicitTextInputSelection(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let value = "hello";
let selectionStart = 1;
let selectionEnd = 5;
const input = {
	id: "Jid.1",
	tagName: "INPUT",
	type: "text",
	getAttribute: function() { return null; }
};
Object.defineProperty(input, "value", {
	get: function() { return value; },
	set: function(v) {
		value = v;
		selectionStart = v.length;
		selectionEnd = v.length;
	},
	enumerable: true,
	configurable: true,
});
Object.defineProperty(input, "selectionStart", {
	get: function() { return selectionStart; },
	set: function(v) { selectionStart = v; },
	enumerable: true,
	configurable: true,
});
Object.defineProperty(input, "selectionEnd", {
	get: function() { return selectionEnd; },
	set: function(v) { selectionEnd = v; },
	enumerable: true,
	configurable: true,
});

document.getElementById = function(id) {
	return id === input.id ? input : null;
};
jawsPerform("Value", input.id, JSON.stringify("say hello!"));
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
		t.Fatalf("implicit text input value/selection = %+v, want selection 5:9", got)
	}
}

func TestJawsJS_PerformRemoveAndReplaceMutateDOMAndReportRemovals(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();
jawsDebug = true;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };

const nodes = {};
function makeNode(id) {
	const node = new Node();
	node.id = id;
	node.tagName = "DIV";
	node.dataset = {};
	node.children = [];
	node.parentElement = null;
	node.hasAttribute = function() { return false; };
	node.addEventListener = function() {};
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
	node.replaceWith = function(fragment) {
		const parent = this.parentElement;
		const idx = parent.children.indexOf(this);
		parent.children[idx] = fragment.root;
		fragment.root.parentElement = parent;
		delete nodes[this.id];
		for (let i = 0; i < this.children.length; i++) {
			delete nodes[this.children[i].id];
		}
		for (let i = 0; i < fragment.managed.length; i++) {
			nodes[fragment.managed[i].id] = fragment.managed[i];
		}
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
const retainedDescendant = makeNode("Jid.6");
const removedDescendant = makeNode("Jid.7");
oldNode.outerHTML = '<section id="Jid.4"><div id="Jid.6"></div><div id="Jid.7"></div></section>';
oldNode.children.push(retainedDescendant, removedDescendant);
retainedDescendant.parentElement = oldNode;
removedDescendant.parentElement = oldNode;
replaceParent.children.push(oldNode);
oldNode.parentElement = replaceParent;
jawsAttach(oldNode);
jawsAttach(retainedDescendant);
jawsAttach(removedDescendant);

document.getElementById = function(id) { return nodes[id] || null; };
let replacementRoot = null;
let replacementRetained = null;
const replacement = '<section id="Jid.4"><div id="Jid.6"></div><div id="Jid.8"></div></section>';
jawsElement = function(html) {
	if (html !== replacement) throw new Error("unexpected replacement " + html);
	replacementRoot = makeNode("Jid.4");
	replacementRetained = makeNode("Jid.6");
	const addedDescendant = makeNode("Jid.8");
	replacementRoot.children.push(replacementRetained, addedDescendant);
	replacementRetained.parentElement = replacementRoot;
	addedDescendant.parentElement = replacementRoot;
	const managed = [replacementRoot, replacementRetained, addedDescendant];
	return {
		root: replacementRoot,
		managed: managed,
		querySelectorAll: function(selector) {
			if (selector === '[id^="' + jawsIdPrefix + '"]') return managed;
			return [];
		},
	};
};

jawsPerform("Remove", "Jid.1", JSON.stringify("Jid.2"));
jawsPerform("Replace", "Jid.4", JSON.stringify(replacement));

process.stdout.write(JSON.stringify({
	parentChildren: parent.children.map(function(child) { return child.id; }),
	replacedChildren: replaceParent.children.map(function(child) { return child.id; }),
	removeFrame: jaws.sent[0] || "",
	replaceRemoveFrame: jaws.sent[1] || "",
	frameCount: jaws.sent.length,
	warnings: warnings,
	childStillRegistered: Boolean(nodes["Jid.2"]),
	rootRecreated: replaceParent.children[0] === replacementRoot && oldNode !== replacementRoot,
}));
`)

	var got struct {
		ParentChildren     []string `json:"parentChildren"`
		ReplacedChildren   []string `json:"replacedChildren"`
		RemoveFrame        string   `json:"removeFrame"`
		ReplaceRemoveFrame string   `json:"replaceRemoveFrame"`
		FrameCount         int      `json:"frameCount"`
		Warnings           []string `json:"warnings"`
		ChildRegistered    bool     `json:"childStillRegistered"`
		RootRecreated      bool     `json:"rootRecreated"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.ParentChildren) != 0 {
		t.Fatalf("Remove left children behind: %+v", got.ParentChildren)
	}
	if !reflect.DeepEqual(got.ReplacedChildren, []string{"Jid.4"}) {
		t.Fatalf("Replace children = %+v, want [Jid.4]", got.ReplacedChildren)
	}
	if got.FrameCount != 2 {
		t.Fatalf("frames = %d, want one Remove frame per operation", got.FrameCount)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("changed Replace warnings = %q, want none", got.Warnings)
	}
	if got.ChildRegistered {
		t.Fatal("removed node is still registered")
	}
	if !got.RootRecreated {
		t.Fatalf("Replace did not insert the replacement DOM: %+v", got)
	}
	msg, ok := wire.Parse([]byte(got.RemoveFrame))
	if !ok {
		t.Fatalf("Remove frame must be parseable by wire.Parse, got %q", got.RemoveFrame)
	}
	if msg.What != what.Remove || msg.Jid != 2 || msg.Data != "Jid.3" {
		t.Fatalf("unexpected removal frame: %+v", msg)
	}
	msg, ok = wire.Parse([]byte(got.ReplaceRemoveFrame))
	if !ok {
		t.Fatalf("Replace removal frame must be parseable by wire.Parse, got %q", got.ReplaceRemoveFrame)
	}
	if msg.What != what.Remove || msg.Jid != 4 || msg.Data != "Jid.7" {
		t.Fatalf("unexpected Replace removal frame: %+v", msg)
	}
}

func TestJawsJS_ReplaceAttachFailureLeavesDOMAndServerInSync(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const oldChild = { id: "Jid.7" };
const old = {
	id: "Jid.4",
	children: [oldChild],
	querySelectorAll: function() { return this.children; },
	replaceWith: function(node) { current = node; },
};
let current = old;
document.getElementById = function(id) { return id === "Jid.4" ? current : null; };

const binding = {
	id: "Jid.9",
	hasAttribute: function(name) { return name === "data-jawsstore"; },
};
const replacement = {
	querySelectorAll: function(selector) {
		return selector === '[id^="' + jawsIdPrefix + '"]' ? [binding] : [];
	},
};
jawsElement = function(html) {
	if (html !== "<x>") throw new Error("unexpected replacement");
	return replacement;
};
const errors = [];
console.error = function(err) { errors.push(String(err)); };
jawsMessage({ data: "Replace\tJid.4\t" + JSON.stringify("<x>") + "\n" });
process.stdout.write(JSON.stringify({
	frames: jaws.sent,
	oldStillMounted: current === old && current.children[0] === oldChild,
	errors: errors,
}));
`)
	var got struct {
		Frames          []string `json:"frames"`
		OldStillMounted bool     `json:"oldStillMounted"`
		Errors          []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.Frames) != 0 || !got.OldStillMounted || len(got.Errors) != 1 ||
		!strings.Contains(got.Errors[0], "JsVar binding outside initial render") {
		t.Fatalf("failed Replace = %+v, want old DOM and no Remove frame", got)
	}
}

func TestJawsJS_AlertAppendsDismissibleElementDirectly(t *testing.T) {
	raw := runJawsJSSnippet(t, `
global.bootstrap = {};
const selectors = [];
const appended = [];
const alertsElem = {
	append: function(elem) { appended.push(elem); }
};
let wrapper;
document.querySelector = function(selector) {
	selectors.push(selector);
	return selector === "[data-jaws-alerts]" ? alertsElem : null;
};
document.getElementById = function(id) {
	throw new Error("unexpected id lookup: " + id);
};
document.createElement = function() {
	wrapper = {};
	Object.defineProperty(wrapper, "innerHTML", {
		set: function(html) { this.firstElementChild = { outerHTML: html }; }
	});
	return wrapper;
};

jawsAlert("success\nSaved");
// Bootstrap dismissal removes the alert itself, so it must be the container child.
process.stdout.write(JSON.stringify({
	selectors: selectors,
	appended: appended.length,
	appendedWrapper: appended[0] === wrapper,
	alertHTML: appended[0] && appended[0].outerHTML
}));
`)

	var got struct {
		Selectors       []string `json:"selectors"`
		Appended        int      `json:"appended"`
		AppendedWrapper bool     `json:"appendedWrapper"`
		AlertHTML       string   `json:"alertHTML"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Selectors, []string{"[data-jaws-alerts]"}) {
		t.Fatalf("alert selectors = %v", got.Selectors)
	}
	if got.Appended != 1 || got.AppendedWrapper {
		t.Fatalf("appended %d elements, wrapper appended = %t", got.Appended, got.AppendedWrapper)
	}
	if !strings.Contains(got.AlertHTML, "Saved") || !strings.Contains(got.AlertHTML, `data-bs-dismiss="alert"`) {
		t.Fatalf("alert HTML = %q", got.AlertHTML)
	}
}

const lostIndicatorStubs = `
const lostIndicatorSelectors = [];
let lostIndicatorElem = null;
let lostIndicatorCreated = "";
let lostIndicatorPrepended = 0;
document.querySelector = function(selector) {
	lostIndicatorSelectors.push(selector);
	return selector === "[data-jaws-lost]" ? lostIndicatorElem : null;
};
document.body = {
	scrollTop: 10,
	prepend: function(elem) {
		lostIndicatorPrepended++;
		lostIndicatorElem = elem;
	}
};
document.documentElement = { scrollTop: 10 };
jawsElement = function(html) {
	lostIndicatorCreated = html;
	return { innerHTML: html };
};
`

func TestJawsJS_LostUsesDataAttributeHook(t *testing.T) {
	raw := runJawsJSSnippet(t, lostIndicatorStubs+`
setTimeout = function() {};
jaws = new Date();

jawsLost();
lostIndicatorElem = { innerHTML: "old" };
jawsLost();
process.stdout.write(JSON.stringify({
	selectors: lostIndicatorSelectors,
	created: lostIndicatorCreated,
	prepended: lostIndicatorPrepended,
	existingHTML: lostIndicatorElem.innerHTML
}));
`)

	var got struct {
		Selectors    []string `json:"selectors"`
		Created      string   `json:"created"`
		Prepended    int      `json:"prepended"`
		ExistingHTML string   `json:"existingHTML"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Selectors, []string{"[data-jaws-lost]", "[data-jaws-lost]"}) {
		t.Fatalf("lost selectors = %v", got.Selectors)
	}
	if got.Prepended != 1 || !strings.Contains(got.Created, "data-jaws-lost") || strings.Contains(got.Created, `id="jaws-lost"`) {
		t.Fatalf("lost indicator creation = %+v", got)
	}
	if !strings.Contains(got.ExistingHTML, "Server connection lost") {
		t.Fatalf("existing lost indicator was not updated: %q", got.ExistingHTML)
	}
}

func TestJawsJS_FailureRecoveryWaitsFiveSeconds(t *testing.T) {
	raw := runJawsJSSnippet(t, lostIndicatorStubs+`
let now = 100000;
Date.now = function() { return now; };
global.performance = { now: function() { return 60000; } };
let reloads = 0;
window.location.reload = function() { reloads++; };
let reconnects = 0;
jawsReconnect = function() { reconnects++; };
const timers = [];
let nextTimer = 1;
setTimeout = function(fn, ms) {
	const timer = { id: nextTimer++, fn: fn, ms: ms };
	timers.push(timer);
	return timer.id;
};

function FakeSocket() {}
WebSocket = FakeSocket;
jaws = new FakeSocket();
jawsFailed();
jawsFailed();
jaws = new Date(now);
const afterFailure = {
	reconnects: reconnects,
	delays: timers.map(function(timer) { return timer.ms; }),
	selectors: lostIndicatorSelectors.length,
	prepended: lostIndicatorPrepended
};

now += 5000;
timers[0].fn();
const afterGrace = {
	reconnects: reconnects,
	delays: timers.map(function(timer) { return timer.ms; }),
	selectors: lostIndicatorSelectors.length,
	prepended: lostIndicatorPrepended,
	created: lostIndicatorCreated
};

// An available server reloads an old page without flashing the banner first.
jawsHandleReconnect({ currentTarget: { status: 204 } });
const afterSuccessfulProbe = {
	reconnects: reconnects,
	delays: timers.map(function(timer) { return timer.ms; }),
	selectors: lostIndicatorSelectors.length,
	prepended: lostIndicatorPrepended,
	reloads: reloads,
	bodyScrollTop: document.body.scrollTop,
	documentScrollTop: document.documentElement.scrollTop
};

// Exercise the failed-probe outcome independently of the stubbed reload above.
reloads = 0;
jawsHandleReconnect({ currentTarget: { status: 0 } });
const afterFailedProbe = {
	reconnects: reconnects,
	delays: timers.map(function(timer) { return timer.ms; }),
	selectors: lostIndicatorSelectors.length,
	prepended: lostIndicatorPrepended,
	created: lostIndicatorCreated,
	reloads: reloads
};

process.stdout.write(JSON.stringify({
	afterFailure: afterFailure,
	afterGrace: afterGrace,
	afterSuccessfulProbe: afterSuccessfulProbe,
	afterFailedProbe: afterFailedProbe
}));
`)

	type state struct {
		Reconnects int    `json:"reconnects"`
		Delays     []int  `json:"delays"`
		Selectors  int    `json:"selectors"`
		Prepended  int    `json:"prepended"`
		Created    string `json:"created"`
		Reloads    int    `json:"reloads"`
		BodyScroll int    `json:"bodyScrollTop"`
		DocScroll  int    `json:"documentScrollTop"`
	}
	var got struct {
		AfterFailure         state `json:"afterFailure"`
		AfterGrace           state `json:"afterGrace"`
		AfterSuccessfulProbe state `json:"afterSuccessfulProbe"`
		AfterFailedProbe     state `json:"afterFailedProbe"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if want := (state{Delays: []int{5000}}); !reflect.DeepEqual(got.AfterFailure, want) {
		t.Fatalf("state after failure = %+v, want %+v", got.AfterFailure, want)
	}
	if want := (state{Reconnects: 1, Delays: []int{5000}}); !reflect.DeepEqual(got.AfterGrace, want) {
		t.Fatalf("state after grace = %+v, want probe without lost indicator %+v", got.AfterGrace, want)
	}
	if want := (state{Reconnects: 1, Delays: []int{5000}, Reloads: 1, BodyScroll: 10, DocScroll: 10}); !reflect.DeepEqual(got.AfterSuccessfulProbe, want) {
		t.Fatalf("state after successful probe = %+v, want reload without indicator %+v", got.AfterSuccessfulProbe, want)
	}
	if got.AfterFailedProbe.Reconnects != 1 ||
		!reflect.DeepEqual(got.AfterFailedProbe.Delays, []int{5000, 5000}) ||
		got.AfterFailedProbe.Selectors != 1 || got.AfterFailedProbe.Prepended != 1 ||
		got.AfterFailedProbe.Reloads != 0 ||
		!strings.Contains(got.AfterFailedProbe.Created, "Server connection lost 5 seconds ago") {
		t.Fatalf("state after failed probe = %+v, want indicator and delayed retry", got.AfterFailedProbe)
	}
}

func TestJawsJS_PagehideInvalidatesDelayedFailureRecovery(t *testing.T) {
	raw := runJawsJSSnippet(t, lostIndicatorStubs+`
global.performance = { now: function() { return 60000; } };
let reloads = 0;
window.location.reload = function() { reloads++; };
const timers = [];
let nextTimer = 1;
setTimeout = function(fn, ms) {
	const timer = { id: nextTimer++, fn: fn, ms: ms };
	timers.push(timer);
	return timer.id;
};

const requests = [];
function FakeXHR() {
	this.status = 0;
	this.listeners = {};
	requests.push(this);
}
FakeXHR.prototype.open = function() {};
FakeXHR.prototype.addEventListener = function(name, fn) {
	this.listeners[name] = fn;
};
FakeXHR.prototype.send = function() {};
XMLHttpRequest = FakeXHR;

function FakeSocket() {}
WebSocket = FakeSocket;
jaws = new FakeSocket();
jawsFailed();
const delayedRecovery = timers[0].fn;

// The committed navigation arrives after the transport failure.
jawsUnloading();
delayedRecovery();
jawsHandleReconnect({ currentTarget: { status: 0 } });
jawsHandleReconnect({ currentTarget: { status: 204 } });
jawsReconnect();

process.stdout.write(JSON.stringify({
	jawsIsNull: jaws === null,
	requests: requests.length,
	delays: timers.map(function(timer) { return timer.ms; }),
	selectors: lostIndicatorSelectors.length,
	prepended: lostIndicatorPrepended,
	reloads: reloads
}));
`)

	var got struct {
		JawsIsNull bool  `json:"jawsIsNull"`
		Requests   int   `json:"requests"`
		Delays     []int `json:"delays"`
		Selectors  int   `json:"selectors"`
		Prepended  int   `json:"prepended"`
		Reloads    int   `json:"reloads"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.JawsIsNull || got.Requests != 0 || !reflect.DeepEqual(got.Delays, []int{5000}) ||
		got.Selectors != 0 || got.Prepended != 0 || got.Reloads != 0 {
		t.Fatalf("state after pagehide = %+v, want delayed work inert", got)
	}
}

func TestJawsJS_ReconnectXHRResultsAreHandledOnce(t *testing.T) {
	raw := runJawsJSSnippet(t, lostIndicatorStubs+`
global.performance = { now: function() { return 60000; } };
function FakeSocket() {}
WebSocket = FakeSocket;
jaws = new FakeSocket();

let reloaded = 0;
window.location.reload = function() { reloaded++; };
let lost = 0;
const originalJawsLost = jawsLost;
jawsLost = function() {
	lost++;
	originalJawsLost();
};

let timers = 0;
const scheduled = [];
setTimeout = function(fn, ms) {
	timers++;
	scheduled.push({ fn: fn, ms: ms });
};
const requests = [];
let sends = 0;
function FakeXHR() {
	this.readyState = 0;
	this.status = 0;
	this.timeout = 0;
	this.timeoutAtSend = 0;
	this.listeners = {};
	requests.push(this);
}
FakeXHR.prototype.open = function(method, url, async) {
	this.method = method;
	this.url = url;
	this.async = async;
};
FakeXHR.prototype.addEventListener = function(name, cb, options) {
	this.listeners[name] = { cb: cb, once: Boolean(options && options.once) };
};
FakeXHR.prototype.send = function() {
	sends++;
	this.timeoutAtSend = this.timeout;
	// Stay pending until the test simulates a browser terminal event.
};
FakeXHR.prototype.dispatch = function(name) {
	const listener = this.listeners[name];
	if (!listener) return;
	if (listener.once) delete this.listeners[name];
	listener.cb({ type: name, currentTarget: this });
};
FakeXHR.prototype.finish = function(name, status) {
	this.readyState = 4;
	this.status = status;
	this.dispatch("readystatechange");
	this.dispatch(name);
	this.dispatch("loadend");
};
XMLHttpRequest = FakeXHR;

jawsFailed();
scheduled[0].fn();
for (let i = 1; i < 5; i++) jawsReconnect();
const pending = { timers: timers, lost: lost, reloaded: reloaded };

requests[0].finish("timeout", 0);
const results = [{ timers: timers, lost: lost, reloaded: reloaded }];
requests[1].finish("error", 0);
results.push({ timers: timers, lost: lost, reloaded: reloaded });
requests[2].finish("abort", 0);
results.push({ timers: timers, lost: lost, reloaded: reloaded });
requests[3].finish("load", 503);
results.push({ timers: timers, lost: lost, reloaded: reloaded });
requests[4].finish("load", 204);
results.push({ timers: timers, lost: lost, reloaded: reloaded });

// One-shot completion prevents stale terminal events from changing the result.
requests.slice(0, 5).forEach(function(req) { req.finish("timeout", 0); });

scheduled[1].fn();

process.stdout.write(JSON.stringify({
	jawsIsDate: jaws instanceof Date,
	sends: sends,
	pending: pending,
	attempts: requests.map(function(req) {
		return {
			method: req.method,
			url: req.url,
			async: req.async,
			timeoutAtSend: req.timeoutAtSend
		};
	}),
	results: results,
	final: { timers: timers, lost: lost, reloaded: reloaded },
	lostHTML: lostIndicatorElem && lostIndicatorElem.innerHTML
}));
`)

	type counts struct {
		Timers   int `json:"timers"`
		Lost     int `json:"lost"`
		Reloaded int `json:"reloaded"`
	}
	type attempt struct {
		Method        string  `json:"method"`
		URL           string  `json:"url"`
		Async         bool    `json:"async"`
		TimeoutAtSend float64 `json:"timeoutAtSend"`
	}
	var got struct {
		JawsIsDate bool      `json:"jawsIsDate"`
		Sends      int       `json:"sends"`
		Pending    counts    `json:"pending"`
		Attempts   []attempt `json:"attempts"`
		Results    []counts  `json:"results"`
		Final      counts    `json:"final"`
		LostHTML   string    `json:"lostHTML"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.JawsIsDate {
		t.Fatal("failed socket did not enter reconnect state")
	}
	if len(got.Attempts) != 6 || got.Sends != 6 {
		t.Fatalf("reconnect attempts = %d requests, %d sends; want 6 each", len(got.Attempts), got.Sends)
	}
	if got.Pending != (counts{Timers: 1}) {
		t.Fatalf("reconnect state before results = %+v, want only the elapsed grace timeout", got.Pending)
	}
	for i, attempt := range got.Attempts {
		if attempt.Method != "GET" || attempt.URL != "http://example.test/jaws/.ping" || !attempt.Async {
			t.Fatalf("reconnect attempt[%d] = %+v, want async GET to /jaws/.ping", i, attempt)
		}
		if attempt.TimeoutAtSend != 10*1000 {
			t.Fatalf("reconnect timeout[%d] = %v, want 10000", i, attempt.TimeoutAtSend)
		}
	}
	wants := []counts{
		{Timers: 2, Lost: 1},
		{Timers: 3, Lost: 2},
		{Timers: 4, Lost: 3},
		{Timers: 5, Lost: 4},
		{Timers: 5, Lost: 4, Reloaded: 1},
	}
	if !reflect.DeepEqual(got.Results, wants) {
		t.Fatalf("reconnect results = %+v, want %+v", got.Results, wants)
	}
	if want := wants[len(wants)-1]; got.Final != want {
		t.Fatalf("reconnect final result = %+v, want %+v", got.Final, want)
	}
	if !strings.Contains(got.LostHTML, "Server connection lost") {
		t.Fatalf("lost indicator = %q", got.LostHTML)
	}
}

func TestJawsJS_ReconnectReloadUsesNavigationAge(t *testing.T) {
	raw := runJawsJSSnippet(t, lostIndicatorStubs+`
let navigationAge = 59999;
global.performance = { now: function() { return navigationAge; } };
jaws = new Date();
let reloads = 0;
window.location.reload = function() { reloads++; };
const delays = [];
setTimeout = function(fn, ms) {
	delays.push(ms);
};

const results = [];
function record(name) {
	results.push({
		name: name,
		reloads: reloads,
		delays: delays.slice(),
		prepended: lostIndicatorPrepended
	});
}

// A page just under the minimum age backs off.
jawsHandleReconnect({ currentTarget: { status: 204 } });
record("before minimum age");

// The exact age boundary permits a reload.
navigationAge = 60000;
jawsHandleReconnect({ currentTarget: { status: 204 } });
record("at minimum age");

// Simulate the fresh navigation, including its new document.
navigationAge = 0;
lostIndicatorElem = null;
jawsHandleReconnect({ currentTarget: { status: 204 } });
record("fresh navigation");

process.stdout.write(JSON.stringify(results));
`)

	type result struct {
		Name      string `json:"name"`
		Reloads   int    `json:"reloads"`
		Delays    []int  `json:"delays"`
		Prepended int    `json:"prepended"`
	}
	var got []result
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	want := []result{
		{Name: "before minimum age", Delays: []int{1000}, Prepended: 1},
		{Name: "at minimum age", Reloads: 1, Delays: []int{1000}, Prepended: 1},
		{Name: "fresh navigation", Reloads: 1, Delays: []int{1000, 1000}, Prepended: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reconnect reload timing = %+v, want %+v", got, want)
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

func TestJawsJS_CanceledBeforeUnloadDefersTeardownToPagehide(t *testing.T) {
	raw := runJawsJSSnippet(t, `
let reconnects = 0;
jawsReconnect = function() { reconnects++; };
let reloads = 0;
window.location.reload = function() { reloads++; };

let appBeforeUnloadCalls = 0;
window.addEventListener("beforeunload", function(event) {
	appBeforeUnloadCalls++;
	event.preventDefault();
	event.returnValue = "stay";
});

function FakeSocket() {
	this.readyState = 1;
	this.closeCount = 0;
	this.listeners = {};
}
FakeSocket.prototype.addEventListener = function(name, fn) {
	(this.listeners[name] ||= []).push(fn);
};
FakeSocket.prototype.removeEventListener = function(name, fn) {
	this.listeners[name] = (this.listeners[name] || []).filter(function(other) {
		return other !== fn;
	});
};
FakeSocket.prototype.listenerCount = function(name) {
	return (this.listeners[name] || []).length;
};
FakeSocket.prototype.dispatch = function(name) {
	(this.listeners[name] || []).slice().forEach(function(fn) {
		fn({ type: name, currentTarget: this });
	}, this);
};
FakeSocket.prototype.close = function() {
	this.closeCount++;
	this.dispatch("close");
};
WebSocket = FakeSocket;

jawsConnect();
const socket = jaws;
const registered = {
	beforeunload: (windowListeners.beforeunload || []).includes(jawsUnloading),
	pagehide: (windowListeners.pagehide || []).includes(jawsUnloading),
	pageshow: (windowListeners.pageshow || []).includes(jawsPageshow)
};

jawsDispatchWindowEvent("pageshow", { type: "pageshow", persisted: false });
const reloadsAfterFreshShow = reloads;

const beforeEvent = {
	type: "beforeunload",
	defaultPrevented: false,
	returnValue: "",
	preventDefault: function() { this.defaultPrevented = true; }
};
jawsDispatchWindowEvent("beforeunload", beforeEvent);
const canceled = {
	appCalls: appBeforeUnloadCalls,
	defaultPrevented: beforeEvent.defaultPrevented,
	sameSocket: jaws === socket,
	canSend: jawsCanSend(),
	closeCount: socket.closeCount,
	closeListeners: socket.listenerCount("close"),
	errorListeners: socket.listenerCount("error"),
	reconnects: reconnects
};

// A later committed navigation reaches pagehide.
jawsDispatchWindowEvent("pagehide", { type: "pagehide", persisted: false });
const hiddenAfterNavigation = {
	jawsIsNull: jaws === null,
	closeCount: socket.closeCount,
	closeListeners: socket.listenerCount("close"),
	errorListeners: socket.listenerCount("error"),
	reconnects: reconnects
};

// Re-arm the transport to exercise a back/forward-cache pagehide independently.
const cachedSocket = new FakeSocket();
cachedSocket.addEventListener("close", jawsFailed);
cachedSocket.addEventListener("error", jawsFailed);
jaws = cachedSocket;
jawsDispatchWindowEvent("pagehide", { type: "pagehide", persisted: true });
const hiddenForCache = {
	jawsIsNull: jaws === null,
	closeCount: cachedSocket.closeCount,
	closeListeners: cachedSocket.listenerCount("close"),
	errorListeners: cachedSocket.listenerCount("error"),
	reconnects: reconnects
};

jawsDispatchWindowEvent("pageshow", { type: "pageshow", persisted: true });

process.stdout.write(JSON.stringify({
	registered: registered,
	canceled: canceled,
	hidden: [hiddenAfterNavigation, hiddenForCache],
	reloadsAfterFreshShow: reloadsAfterFreshShow,
	reloads: reloads
}));
`)

	var got struct {
		Registered struct {
			BeforeUnload bool `json:"beforeunload"`
			Pagehide     bool `json:"pagehide"`
			Pageshow     bool `json:"pageshow"`
		} `json:"registered"`
		Canceled struct {
			AppCalls         int  `json:"appCalls"`
			DefaultPrevented bool `json:"defaultPrevented"`
			SameSocket       bool `json:"sameSocket"`
			CanSend          bool `json:"canSend"`
			CloseCount       int  `json:"closeCount"`
			CloseListeners   int  `json:"closeListeners"`
			ErrorListeners   int  `json:"errorListeners"`
			Reconnects       int  `json:"reconnects"`
		} `json:"canceled"`
		Hidden []struct {
			JawsIsNull     bool `json:"jawsIsNull"`
			CloseCount     int  `json:"closeCount"`
			CloseListeners int  `json:"closeListeners"`
			ErrorListeners int  `json:"errorListeners"`
			Reconnects     int  `json:"reconnects"`
		} `json:"hidden"`
		ReloadsAfterFreshShow int `json:"reloadsAfterFreshShow"`
		Reloads               int `json:"reloads"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Registered.BeforeUnload || !got.Registered.Pagehide || !got.Registered.Pageshow {
		t.Fatalf("lifecycle listeners = %+v, want pagehide and pageshow only", got.Registered)
	}
	if got.Canceled.AppCalls != 1 || !got.Canceled.DefaultPrevented || !got.Canceled.SameSocket || !got.Canceled.CanSend {
		t.Fatalf("canceled navigation state = %+v, want the original live socket", got.Canceled)
	}
	if got.Canceled.CloseCount != 0 || got.Canceled.CloseListeners != 1 || got.Canceled.ErrorListeners != 1 || got.Canceled.Reconnects != 0 {
		t.Fatalf("canceled navigation transport = %+v, want untouched failure handling", got.Canceled)
	}
	if len(got.Hidden) != 2 {
		t.Fatalf("pagehide results = %d, want 2", len(got.Hidden))
	}
	for i, hidden := range got.Hidden {
		if !hidden.JawsIsNull || hidden.CloseCount != 1 || hidden.CloseListeners != 0 || hidden.ErrorListeners != 0 || hidden.Reconnects != 0 {
			t.Fatalf("pagehide result %d = %+v, want one intentional close without reconnect", i, hidden)
		}
	}
	if got.ReloadsAfterFreshShow != 0 || got.Reloads != 1 {
		t.Fatalf("pageshow reloads = %d before persisted, %d total; want 0 and 1", got.ReloadsAfterFreshShow, got.Reloads)
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

func TestJawsJS_RemovePositionRequiresDirectChildJid(t *testing.T) {
	raw := runJawsJSSnippet(t, `
console.log = function() {};
const childLookups = [];
const removed = [];
const parent = new Node();
parent.id = "Jid.1";
parent.children = [];
parent.removeChild = function(child) {
	removed.push(child.id);
	child.parentElement = null;
};

function makeChild(id, owner) {
	const child = new Node();
	child.id = id;
	child.parentElement = owner;
	child.querySelectorAll = function() { return []; };
	return child;
}

const canonical = makeChild("Jid.2", parent);
const arbitrary = makeChild("application-id", parent);
const noncanonical = makeChild("Jid.03", parent);
const unrelated = makeChild("Jid.4", {});
parent.children = [canonical, arbitrary, noncanonical];
const nodes = {
	"Jid.1": parent,
	"Jid.2": canonical,
	"application-id": arbitrary,
	"Jid.03": noncanonical,
	"Jid.4": unrelated
};
document.getElementById = function(id) {
	if (id !== "Jid.1") childLookups.push(id);
	return nodes[id] || null;
};

["0", "null", "application-id", "Jid.03", "Jid.4", "Jid.2"].forEach(function(pos) {
	jawsPerform("Remove", "Jid.1", JSON.stringify(pos));
});
process.stdout.write(JSON.stringify({ childLookups: childLookups, removed: removed }));
`)

	var got struct {
		ChildLookups []string `json:"childLookups"`
		Removed      []string `json:"removed"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.ChildLookups, []string{"Jid.4", "Jid.2"}) {
		t.Fatalf("Remove child lookups = %v", got.ChildLookups)
	}
	if !reflect.DeepEqual(got.Removed, []string{"Jid.2"}) {
		t.Fatalf("removed children = %v", got.Removed)
	}
}

func TestJawsJS_InsertPositionRequiresDirectChildJidOrIndex(t *testing.T) {
	raw := runJawsJSSnippet(t, `
console.log = function() {};
const lookups = [];
const parent = { id: "Jid.1", children: [] };
const child = new Node();
child.id = "Jid.2";
child.parentElement = parent;
const unrelated = new Node();
unrelated.id = "Jid.3";
unrelated.parentElement = {};
parent.children = [child];
document.getElementById = function(id) {
	lookups.push(id);
	if (id === "Jid.2") return child;
	if (id === "Jid.3") return unrelated;
	return null;
};

const positions = ["Jid.2", "0", "Jid.3", "null", "-1", "Jid.02", "application-id"];
const resolved = positions.map(function(pos) {
	const elem = jawsInsertWhere(parent, pos);
	return elem ? elem.id : "";
});
process.stdout.write(JSON.stringify({ lookups: lookups, resolved: resolved }));
`)

	var got struct {
		Lookups  []string `json:"lookups"`
		Resolved []string `json:"resolved"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got.Lookups, []string{"Jid.2", "Jid.3"}) {
		t.Fatalf("Insert child lookups = %v", got.Lookups)
	}
	want := []string{"Jid.2", "Jid.2", "", "", "", "", ""}
	if !reflect.DeepEqual(got.Resolved, want) {
		t.Fatalf("Insert position results = %v, want %v", got.Resolved, want)
	}
}

func TestJawsJS_InsertNullPositionIsRejected(t *testing.T) {
	raw := runJawsJSSnippet(t, `
console.log = function() {};
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
	if got.Inserted {
		t.Fatal(`Insert with position "null" should be rejected; callers use Append for end insertion`)
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

func TestJawsJS_InsertArbitraryIDIsRejected(t *testing.T) {
	raw := runJawsJSSnippet(t, `
console.log = function() {};
let inserted = false;
const newChild = {
	id: "new-child",
	querySelectorAll: function() { return { forEach: function() {} }; }
};
const existingChild = { id: "existing-child" };
Object.setPrototypeOf(newChild, Node.prototype);
Object.setPrototypeOf(existingChild, Node.prototype);
const elem = {
	id: "Jid.1",
	children: [existingChild],
	insertBefore: function() {
		inserted = true;
	}
};
existingChild.parentElement = elem;
document.getElementById = function(id) {
	if (id === "Jid.1") return elem;
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

jawsPerform("Insert", "Jid.1", JSON.stringify("0-panel\n<span id=\"new-child\"></span>"));
process.stdout.write(JSON.stringify({ inserted: inserted }));
`)

	var got struct {
		Inserted bool `json:"inserted"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if got.Inserted {
		t.Fatal(`Insert position "0-panel" should be rejected, not treated as child index 0`)
	}
}

func TestJawsJS_OrderPreservesApplicationDataset(t *testing.T) {
	raw := runJawsJSSnippet(t, `
const parent = { appended: [], appendChild: function(elem) { this.appended.push(elem.id); } };
const one = { id: "Jid.1", dataset: { jidsort: "application-one" }, parentElement: parent };
const two = { id: "Jid.2", dataset: { jidsort: "application-two" }, parentElement: parent };
const lookups = [];
document.getElementById = function(id) {
	lookups.push(id);
	if (id === "Jid.1") return one;
	if (id === "Jid.2") return two;
	return null;
};

jawsPerform("Order", "", JSON.stringify("Jid.2 application-id Jid.01 Jid.0 Jid.1"));
process.stdout.write(JSON.stringify({
	appended: parent.appended,
	lookups: lookups,
	oneSort: one.dataset.jidsort || "",
	twoSort: two.dataset.jidsort || ""
}));
`)

	var got struct {
		Appended []string `json:"appended"`
		Lookups  []string `json:"lookups"`
		OneSort  string   `json:"oneSort"`
		TwoSort  string   `json:"twoSort"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if strings.Join(got.Appended, " ") != "Jid.2 Jid.1" {
		t.Fatalf("unexpected append order: %#v", got.Appended)
	}
	if !reflect.DeepEqual(got.Lookups, []string{"Jid.2", "Jid.1"}) {
		t.Fatalf("order lookups = %v, want only canonical Jids", got.Lookups)
	}
	if got.OneSort != "application-one" || got.TwoSort != "application-two" {
		t.Fatalf("jawsOrder clobbered application data-jidsort: got %q and %q", got.OneSort, got.TwoSort)
	}
}

func TestJawsJS_InnerWarnsWhenDebugEnabledAndHTMLUnchanged(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = true;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
let writes = 0;
const elem = {
	id: "Jid.1",
	querySelectorAll: function() { return []; },
};
Object.defineProperty(elem, "innerHTML", {
	get: function() { return "<b>x</b>"; },
	set: function() { writes++; },
});
document.getElementById = function(id) { return id === "Jid.1" ? elem : null; };

jawsPerform("Inner", "Jid.1", JSON.stringify("<b>x</b>"));
process.stdout.write(JSON.stringify({ warnings: warnings, innerHTML: elem.innerHTML, writes: writes }));
`)

	var got struct {
		Warnings  []string `json:"warnings"`
		InnerHTML string   `json:"innerHTML"`
		Writes    int      `json:"writes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got.Warnings))
	}
	if got.Warnings[0] != "jaws: Inner Jid.1: requested HTML matches the current serialized HTML; the DOM update was skipped" {
		t.Fatalf("unexpected warning text %q", got.Warnings[0])
	}
	if got.InnerHTML != "<b>x</b>" {
		t.Fatalf("innerHTML = %q, want unchanged", got.InnerHTML)
	}
	if got.Writes != 0 {
		t.Fatalf("innerHTML writes = %d, want 0", got.Writes)
	}
}

func TestJawsJS_ReplaceRecreatesNodeWhenHTMLUnchanged(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = true;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const replacement = '<div id="Jid.1"><input id="Jid.2" type="text" value="server"><input id="Jid.3" type="checkbox" checked></div>';
const beforeInput = {
	id: "Jid.2",
	tagName: "INPUT",
	type: "text",
	value: "client edit",
	dataset: {},
	hasAttribute: function() { return false; },
};
const beforeCheckbox = {
	id: "Jid.3",
	tagName: "INPUT",
	type: "checkbox",
	checked: false,
	dataset: {},
	hasAttribute: function() { return false; },
};
let outerHTMLReads = 0;
const before = {
	id: "Jid.1",
	tagName: "DIV",
	querySelectorAll: function(selector) {
		if (selector === '[id^="' + jawsIdPrefix + '"]') return [beforeInput, beforeCheckbox];
		return [];
	},
	replaceWith: function(fragment) {
		current = fragment.root;
		currentInput = fragment.input;
		currentCheckbox = fragment.checkbox;
	},
};
Object.defineProperty(before, "outerHTML", {
	get: function() { outerHTMLReads++; return replacement; },
});
let current = before;
let currentInput = beforeInput;
let currentCheckbox = beforeCheckbox;
document.getElementById = function(id) {
	if (id === "Jid.1") return current;
	if (id === "Jid.2") return currentInput;
	if (id === "Jid.3") return currentCheckbox;
	return null;
};
document.createElement = function(tag) {
	if (tag !== "template") throw new Error("unexpected tag " + tag);
	const afterInput = {
		id: "Jid.2",
		tagName: "INPUT",
		type: "text",
		value: "server",
		dataset: {},
		listeners: [],
		hasAttribute: function() { return false; },
		addEventListener: function(name) { this.listeners.push(name); },
		querySelectorAll: function() { return []; },
	};
	const afterCheckbox = {
		id: "Jid.3",
		tagName: "INPUT",
		type: "checkbox",
		checked: true,
		dataset: {},
		listeners: [],
		hasAttribute: function() { return false; },
		addEventListener: function(name) { this.listeners.push(name); },
		querySelectorAll: function() { return []; },
	};
	const after = {
		id: "Jid.1",
		tagName: "DIV",
		listeners: [],
		hasAttribute: function() { return false; },
		addEventListener: function(name) { this.listeners.push(name); },
		querySelectorAll: function(selector) {
			if (selector === '[id^="' + jawsIdPrefix + '"]') return [afterInput, afterCheckbox];
			return [];
		},
	};
	const fragment = {
		root: after,
		input: afterInput,
		checkbox: afterCheckbox,
		querySelectorAll: function(selector) {
			if (selector === '[id^="' + jawsIdPrefix + '"]') return [after, afterInput, afterCheckbox];
			return [];
		},
	};
	const template = {
		content: fragment,
	};
	Object.defineProperty(template, "innerHTML", {
		set: function(v) {
			if (v !== replacement) throw new Error("unexpected replacement " + v);
		},
		enumerable: true,
		configurable: true,
	});
	return template;
};

jawsPerform("Replace", "Jid.1", JSON.stringify(replacement));
const after = document.getElementById("Jid.1");
const afterInput = document.getElementById("Jid.2");
const afterCheckbox = document.getElementById("Jid.3");
process.stdout.write(JSON.stringify({
	warnings: warnings,
	outerHTMLReads: outerHTMLReads,
	sameNode: before === after,
	id: after.id,
	sameInput: beforeInput === afterInput,
	sameCheckbox: beforeCheckbox === afterCheckbox,
	inputID: afterInput.id,
	value: afterInput.value,
	inputListeners: afterInput.listeners,
	checked: afterCheckbox.checked,
	checkboxListeners: afterCheckbox.listeners,
	frames: jaws.sent,
}));
`)

	var got struct {
		Warnings          []string `json:"warnings"`
		OuterHTMLReads    int      `json:"outerHTMLReads"`
		SameNode          bool     `json:"sameNode"`
		ID                string   `json:"id"`
		SameInput         bool     `json:"sameInput"`
		SameCheckbox      bool     `json:"sameCheckbox"`
		InputID           string   `json:"inputID"`
		Value             string   `json:"value"`
		InputListeners    []string `json:"inputListeners"`
		Checked           bool     `json:"checked"`
		CheckboxListeners []string `json:"checkboxListeners"`
		Frames            []string `json:"frames"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "jaws: Replace Jid.1: requested HTML matches the current serialized HTML; the DOM node is still recreated" {
		t.Fatalf("warnings = %q, want one recreation warning", got.Warnings)
	}
	if got.OuterHTMLReads != 1 {
		t.Fatalf("outerHTML reads = %d, want 1 in debug mode", got.OuterHTMLReads)
	}
	if got.SameNode {
		t.Fatal("Replace kept the existing DOM node")
	}
	if got.ID != "Jid.1" {
		t.Fatalf("replacement id = %q, want %q", got.ID, "Jid.1")
	}
	if got.SameInput {
		t.Fatal("Replace kept the existing managed input")
	}
	if got.SameCheckbox {
		t.Fatal("Replace kept the existing managed checkbox")
	}
	if got.InputID != "Jid.2" {
		t.Fatalf("replacement input id = %q, want %q", got.InputID, "Jid.2")
	}
	if got.Value != "server" {
		t.Fatalf("replacement input value = %q, want server", got.Value)
	}
	if !reflect.DeepEqual(got.InputListeners, []string{"input"}) {
		t.Fatalf("replacement input listeners = %q, want [input]", got.InputListeners)
	}
	if !got.Checked {
		t.Fatal("replacement checkbox did not use the checked markup state")
	}
	if !reflect.DeepEqual(got.CheckboxListeners, []string{"input"}) {
		t.Fatalf("replacement checkbox listeners = %q, want [input]", got.CheckboxListeners)
	}
	if len(got.Frames) != 0 {
		t.Fatalf("frames = %q, want none", got.Frames)
	}
}

func TestJawsJS_ReplaceRetainsJidsAcrossBrowserSerialization(t *testing.T) {
	raw := runJawsJSSnippet(t, `
jawsDebug = false;
const warnings = [];
console.warn = function(msg) { warnings.push(msg); };
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

const source = '<button id="Jid.1" disabled><span id="Jid.2"></span></button>';
const beforeChild = { id: "Jid.2", dataset: {} };
let outerHTMLReads = 0;
const before = {
	id: "Jid.1",
	dataset: {},
	hasAttribute: function() { return false; },
	querySelectorAll: function(selector) {
		if (selector === '[id^="' + jawsIdPrefix + '"]') return [beforeChild];
		return [];
	},
	replaceWith: function(fragment) {
		current = fragment.root;
		currentChild = fragment.child;
	},
};
Object.defineProperty(before, "outerHTML", {
	get: function() {
		outerHTMLReads++;
		return '<button id="Jid.1" disabled=""><span id="Jid.2"></span></button>';
	},
});

const afterChild = {
	id: "Jid.2",
	tagName: "SPAN",
	dataset: {},
	hasAttribute: function() { return false; },
	addEventListener: function() {},
};
const after = {
	id: "Jid.1",
	tagName: "BUTTON",
	dataset: {},
	hasAttribute: function() { return false; },
	addEventListener: function() {},
};
const fragment = {
	root: after,
	child: afterChild,
	querySelectorAll: function(selector) {
		if (selector === '[id^="' + jawsIdPrefix + '"]') return [after, afterChild];
		return [];
	},
};
let current = before;
let currentChild = beforeChild;
document.getElementById = function(id) {
	if (id === "Jid.1") return current;
	if (id === "Jid.2") return currentChild;
	return null;
};
jawsElement = function(html) {
	if (html !== source) throw new Error("unexpected replacement " + html);
	return fragment;
};

jawsPerform("Replace", "Jid.1", JSON.stringify(source));
process.stdout.write(JSON.stringify({
	rootRecreated: current === after && current !== before,
	childRecreated: currentChild === afterChild && currentChild !== beforeChild,
	outerHTMLReads: outerHTMLReads,
	warnings: warnings,
	frames: jaws.sent,
}));
`)

	var got struct {
		RootRecreated  bool     `json:"rootRecreated"`
		ChildRecreated bool     `json:"childRecreated"`
		OuterHTMLReads int      `json:"outerHTMLReads"`
		Warnings       []string `json:"warnings"`
		Frames         []string `json:"frames"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	if !got.RootRecreated || !got.ChildRecreated {
		t.Fatalf("Replace did not recreate browser-normalized markup: %+v", got)
	}
	if got.OuterHTMLReads != 0 {
		t.Fatalf("outerHTML reads = %d, want none outside debug mode", got.OuterHTMLReads)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %q, want none outside debug mode", got.Warnings)
	}
	if len(got.Frames) != 0 {
		t.Fatalf("Replace reported retained descendants as removed: %q", got.Frames)
	}
}

func TestJawsJS_AttrGuardsRejectReservedId(t *testing.T) {
	raw := runJawsJSSnippet(t, `
function FakeSocket() { this.readyState = 1; this.sent = []; }
FakeSocket.prototype.send = function(msg) { this.sent.push(msg); };
WebSocket = FakeSocket;
jaws = new FakeSocket();

// Keep the Inner order in the frame test focused on "did it run at all".
jawsRemoving = function() {};
jawsAttachChildren = function(node) { return node; };

const elem1 = {
	id: "Jid.1",
	attrs: { id: "Jid.1" },
	getAttribute: function(a) { return Object.prototype.hasOwnProperty.call(this.attrs, a) ? this.attrs[a] : null; },
	setAttribute: function(a, v) { this.attrs[a] = v; },
	removeAttribute: function(a) { delete this.attrs[a]; },
	innerHTML: "",
	querySelectorAll: function() { return []; },
};
const elem2 = {
	id: "Jid.2",
	innerHTML: "old",
	querySelectorAll: function() { return []; },
};
const elems = { "Jid.1": elem1, "Jid.2": elem2 };
document.getElementById = function(id) { return elems[id] || null; };

const errors = [];
console.error = function(msg) { errors.push(String(msg)); };

// SAttr rejects "id" ASCII case-insensitively without touching setAttribute.
const sattrThrows = {};
["id", "ID", "Id", "iD"].forEach(function(name) {
	try {
		jawsSetAttr(elem1, name + "\nhacked");
		sattrThrows[name] = false;
	} catch (err) {
		sattrThrows[name] = true;
	}
});
// A normal attribute is still applied.
jawsSetAttr(elem1, "title\nhello");
const normalAttr = elem1.getAttribute("title");

// RAttr rejects "id" ASCII case-insensitively without touching removeAttribute.
const rattrThrows = {};
["id", "ID", "Id", "iD"].forEach(function(name) {
	try {
		jawsPerform("RAttr", "Jid.1", JSON.stringify(name));
		rattrThrows[name] = false;
	} catch (err) {
		rattrThrows[name] = true;
	}
});
const idAfterRemove = elem1.getAttribute("id");

// A frame whose first order tries to change "id" throws, but jawsMessage isolates
// it so later orders in the same frame still apply.
const frame = [
	"SAttr\tJid.1\t" + JSON.stringify("id\nhacked"),
	"Inner\tJid.2\t" + JSON.stringify("<b>ok</b>")
].join("\n");
jawsMessage({ data: frame });

process.stdout.write(JSON.stringify({
	sattrThrows: sattrThrows,
	rattrThrows: rattrThrows,
	normalAttr: normalAttr,
	idAfterRemove: idAfterRemove,
	idAfterFrame: elem1.getAttribute("id"),
	laterOrderInner: elem2.innerHTML,
	frameErrors: errors.length
}));
`)

	var got struct {
		SattrThrows     map[string]bool `json:"sattrThrows"`
		RattrThrows     map[string]bool `json:"rattrThrows"`
		NormalAttr      string          `json:"normalAttr"`
		IDAfterRemove   string          `json:"idAfterRemove"`
		IDAfterFrame    string          `json:"idAfterFrame"`
		LaterOrderInner string          `json:"laterOrderInner"`
		FrameErrors     int             `json:"frameErrors"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &got); err != nil {
		t.Fatalf("failed to parse snippet output %q: %v", raw, err)
	}
	for _, name := range []string{"id", "ID", "Id", "iD"} {
		if !got.SattrThrows[name] {
			t.Errorf("jawsSetAttr(%q) did not reject the reserved attribute", name)
		}
		if !got.RattrThrows[name] {
			t.Errorf("RAttr %q did not reject the reserved attribute", name)
		}
	}
	if got.NormalAttr != "hello" {
		t.Errorf("normal attribute = %q, want %q", got.NormalAttr, "hello")
	}
	if got.IDAfterRemove != "Jid.1" {
		t.Errorf("id after rejected RemoveAttr = %q, want it untouched", got.IDAfterRemove)
	}
	if got.IDAfterFrame != "Jid.1" {
		t.Errorf("id after rejected SAttr in frame = %q, want it untouched", got.IDAfterFrame)
	}
	if got.LaterOrderInner != "<b>ok</b>" {
		t.Errorf("later order in frame = %q, want it applied despite the earlier throw", got.LaterOrderInner)
	}
	if got.FrameErrors == 0 {
		t.Error("rejected SAttr in frame was not surfaced via console.error")
	}
}
