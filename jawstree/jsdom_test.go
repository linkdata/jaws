package jawstree

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// jawstreeJsdomRunner loads the page given as its argument under jsdom and exits
// zero only once the page has turned its #result element green. On failure it
// reports the element's background and any jsdom errors for diagnosis.
const jawstreeJsdomRunner = `
const { JSDOM, VirtualConsole } = require("jsdom");
const { readFileSync } = require("fs");
const { pathToFileURL } = require("url");
const htmlPath = process.argv[1];
const errors = [];
const vc = new VirtualConsole();
vc.on("jsdomError", function (e) { errors.push(String((e && e.message) || e)); });
const dom = new JSDOM(readFileSync(htmlPath, "utf8"), {
	url: pathToFileURL(htmlPath).href,
	runScripts: "dangerously",
	resources: "usable",
	pretendToBeVisual: true,
	virtualConsole: vc,
});
const window = dom.window;
const deadline = Date.now() + 10000;
const timer = setInterval(function () {
	const result = window.document.getElementById("result");
	const bg = result ? result.style.background.replace(/\s/g, "") : "";
	const green = bg === "rgb(0,255,0)";
	if (green || errors.length > 0 || Date.now() > deadline) {
		clearInterval(timer);
		if (!green) {
			console.error("#result background = " + JSON.stringify(bg) + "; jsdom errors = " + JSON.stringify(errors));
		}
		process.exit(green ? 0 : 1);
	}
}, 25);
`

// jawstreeNodeWithJsdom returns the node executable and the environment needed
// for require("jsdom") to resolve: nil when jsdom is reachable from this package
// directory (an ancestor node_modules), or one with NODE_PATH pointing at the
// npm global root. It skips the test when node or jsdom is unavailable, or fails
// instead when JAWS_REQUIRE_NODE is set.
func jawstreeNodeWithJsdom(t *testing.T) (node string, env []string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err == nil {
		if exec.Command(node, "-e", `require.resolve("jsdom")`).Run() == nil {
			return node, nil
		}
		if npm, e := exec.LookPath("npm"); e == nil {
			if out, e := exec.Command(npm, "root", "-g").Output(); e == nil {
				env = append(os.Environ(), "NODE_PATH="+strings.TrimSpace(string(out)))
				cmd := exec.Command(node, "-e", `require.resolve("jsdom")`)
				cmd.Env = env
				if cmd.Run() == nil {
					return node, env
				}
			}
		}
	}
	if os.Getenv("JAWS_REQUIRE_NODE") != "" {
		t.Fatal("node with the jsdom package not available but JAWS_REQUIRE_NODE is set")
	}
	t.Skip("node with the jsdom package is required (npm install jsdom)")
	return
}

// jawstreeRunJsdomPage loads htmlPath under jsdom and fails the test unless the
// page turns its #result element green within the runner's deadline.
func jawstreeRunJsdomPage(t *testing.T, htmlPath string) {
	t.Helper()
	node, env := jawstreeNodeWithJsdom(t)
	cmd := exec.CommandContext(t.Context(), node, "-e", jawstreeJsdomRunner, htmlPath)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jsdom: %v\n%s", err, output)
	}
}
