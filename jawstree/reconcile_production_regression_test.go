package jawstree

import (
	"image/color"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestReconcileWithRealWidget drives jawstreeReconcile against the actual vendored
// Quercus widget (treeview.js) in headless Firefox, not the test mock, covering the
// collateral-effect cases that broke the earlier one-pass reconcile: a single-select
// switch keeping only the new node, a cascade parent-deselect keeping the still-desired
// children, cascade select-all, and empty-desired. The page turns a fixed pixel green
// only if every assertion against the real DOM passes.
func TestReconcileWithRealWidget(t *testing.T) {
	treeviewJS, err := assetsFS.ReadFile("assets/treeview.js")
	if err != nil {
		t.Fatal(err)
	}
	jawstreeJS, err := assetsFS.ReadFile("assets/jawstree.js")
	if err != nil {
		t.Fatal(err)
	}

	// jawstreeSend is a no-op without a jaws socket, so no jaws.js is needed; the
	// scenarios only push server->client jawstreeSelection frames and read the DOM.
	htmlText := `<!doctype html><html><head>
<style>#result{position:fixed;inset:0;z-index:9999;background:rgb(255,0,0)}</style>
<script>` + string(treeviewJS) + `</script>
<script>` + string(jawstreeJS) + `</script>
</head><body>
<div id="treeS"></div>
<div id="treeC"></div>
<div id="result"></div>
<script>
window.addEventListener("DOMContentLoaded", function () {
	function ids(jid) {
		return Array.from(document.getElementById(jid).querySelectorAll("li.selected"))
			.map(function (li) { return li.dataset.id; }).sort();
	}
	function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
	var ok = true;
	function ck(c) { if (!c) { ok = false; } }

	// Single-select: switching A -> B must leave only B (not clear everything).
	jawstreeInit({ key: "s", jid: "treeS", options: 0, data: { children: [
		{ id: "children.0", name: "A" }, { id: "children.1", name: "B" }
	] } });
	jawstreeSelection({ key: "s", jid: "treeS", s: [1] }); ck(eq(ids("treeS"), ["children.0"]));
	jawstreeSelection({ key: "s", jid: "treeS", s: [2] }); ck(eq(ids("treeS"), ["children.1"]));
	jawstreeSelection({ key: "s", jid: "treeS", s: [] });  ck(eq(ids("treeS"), []));

	// Multi + cascade: deselecting the parent server-side must keep the children.
	jawstreeInit({ key: "c", jid: "treeC", options: (1 << 2) | (1 << 7), data: { children: [
		{ id: "children.0", name: "P", children: [
			{ id: "children.0.children.0", name: "c1" },
			{ id: "children.0.children.1", name: "c2" }
		] }
	] } });
	jawstreeSelection({ key: "c", jid: "treeC", s: [1, 2, 3] });
	ck(eq(ids("treeC"), ["children.0", "children.0.children.0", "children.0.children.1"]));
	jawstreeSelection({ key: "c", jid: "treeC", s: [2, 3] });
	ck(eq(ids("treeC"), ["children.0.children.0", "children.0.children.1"]));
	jawstreeSelection({ key: "c", jid: "treeC", s: [] });
	ck(eq(ids("treeC"), []));

	if (ok) { document.getElementById("result").style.background = "rgb(0,255,0)"; }
});
</script>
</body></html>`

	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "reconcile.html")
	screenshotPath := filepath.Join(dir, "reconcile.png")
	profilePath := filepath.Join(dir, "firefox-profile")
	if err := os.WriteFile(htmlPath, []byte(htmlText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(profilePath, 0o700); err != nil {
		t.Fatal(err)
	}

	pageURL := (&url.URL{Scheme: "file", Path: htmlPath}).String()
	cmd := exec.CommandContext(t.Context(), jawstreeFirefoxPath(t),
		"--headless", "--new-instance", "--profile", profilePath,
		"--screenshot", screenshotPath, "--window-size", "64,64", pageURL)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Firefox failed: %v\n%s", err, output)
	}

	f, err := os.Open(screenshotPath)
	if err != nil {
		t.Fatal(err)
	}
	img, decodeErr := png.Decode(f)
	closeErr := f.Close()
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	pixel := color.RGBAModel.Convert(img.At(32, 32)).(color.RGBA)
	if pixel.R > 32 || pixel.G < 224 || pixel.B > 32 {
		t.Fatalf("reconcile against the real widget failed; center pixel = %#v, want green", pixel)
	}
}
