package jaws

import (
	"bytes"
	_ "embed"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/jaws/what"
)

func Test_PreloadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	const extraImage = "favicon.png"
	const extraFont = "someExtraFont.woff2"
	th := newTestHelper(t)

	serveJS, err := staticserve.New("/jaws/.jaws.js", JavascriptText)
	th.NoErr(err)

	txt, fav := PreloadHTML()
	th.Equal(strings.Contains(txt, serveJS.Name), false)
	th.Equal(strings.Count(txt, "<script>"), strings.Count(txt, "</script>"))
	th.Equal(fav, "")

	mustParseUrl := func(urlstr string) *url.URL {
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	txt, fav = PreloadHTML(
		mustParseUrl(serveJS.Name),
		mustParseUrl(extraScript),
		mustParseUrl(extraStyle),
		mustParseUrl(extraImage),
		mustParseUrl(extraFont))
	th.Equal(strings.Contains(txt, serveJS.Name), true)
	th.Equal(strings.Contains(txt, extraScript), true)
	th.Equal(strings.Contains(txt, extraStyle), true)
	th.Equal(strings.Contains(txt, extraImage), true)
	th.Equal(strings.Contains(txt, extraFont), true)
	th.Equal(strings.Count(txt, "<script"), strings.Count(txt, "</script>"))
	th.Equal(fav, extraImage)
	t.Log(txt)
}

func TestJawsKeyString(t *testing.T) {
	th := newTestHelper(t)
	th.Equal(JawsKeyString(0), "")
	th.Equal(JawsKeyString(1), "1")
}

func TestJawsKeyValue(t *testing.T) {
	tests := []struct {
		name    string
		jawsKey string
		want    uint64
	}{
		{
			name:    "blank",
			jawsKey: "",
			want:    0,
		},
		{
			name:    "1",
			jawsKey: "1",
			want:    1,
		},
		{
			name:    "-1",
			jawsKey: "-1",
			want:    0,
		},
		{
			name:    "2/",
			jawsKey: "2/",
			want:    2,
		},
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
	querySelector: function(){ return { content: "123" }; },
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

	msg, ok := wsParse([]byte(raw))
	if !ok {
		t.Fatalf("Set frame must be parseable by core wsParse, got %q", raw)
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

	if msg, ok := wsParse([]byte(raw)); ok {
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
		if _, ok := wsParse([]byte(raw)); !ok {
			t.Fatalf("jawsVar should not emit unparseable Set frame when JsVar name is unregistered, got %q", raw)
		}
	}
}
