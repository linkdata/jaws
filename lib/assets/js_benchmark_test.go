package assets

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func BenchmarkJawsJSMessageDispatch(b *testing.B) {
	node, err := exec.LookPath("node")
	if err != nil {
		b.Skip("node executable not available")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("runtime.Caller failed")
	}
	jsPath := filepath.Join(filepath.Dir(file), "jaws.js")
	const frame = "RAttr\tJid.1\t\"title\"\n"
	const script = `
const fs = require("fs");
const src = fs.readFileSync(process.argv[1], "utf8");
const elem = { id: "Jid.1", removeAttribute: function() {} };
global.window = {
    location: { protocol: "http:", host: "example.test" },
    addEventListener: function() {},
    jawsNames: {}
};
global.document = {
    readyState: "loading",
    querySelector: function(selector) {
        return selector === 'meta[name="jawsKey"]' ? { content: "123" } : null;
    },
    querySelectorAll: function() { return { forEach: function() {} }; },
    getElementById: function(id) { return id === elem.id ? elem : null; }
};
global.XMLHttpRequest = function() {};
global.Event = function() {};
global.Node = function() {};
global.WebSocket = function() {};
eval(src);
if (typeof jawsSetDocumentReady === "function") {
    jawsSetDocumentReady();
}
const event = { data: "RAttr\tJid.1\t\"title\"\n" };
const count = Number(process.argv[2]);
for (let i = 0; i < count; i++) {
    jawsMessage(event);
}
`

	cmd := exec.CommandContext(b.Context(), node, "-e", script, jsPath, strconv.Itoa(b.N))
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()
	if output, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("node failed: %v\n%s", err, output)
	}
}
