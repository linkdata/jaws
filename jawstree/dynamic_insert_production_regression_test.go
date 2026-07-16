package jawstree

import (
	"encoding/json"
	"html/template"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	jawsassets "github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type dynamicTreeContainer struct {
	contents []jaws.UI
}

func (c *dynamicTreeContainer) JawsContains(*jaws.Element) []jaws.UI {
	return c.contents
}

func jawstreeFirefoxPath(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"firefox", "firefox-esr"} {
		if filename, err := exec.LookPath(name); err == nil {
			return filename
		}
	}
	if runtime.GOOS == "darwin" {
		const filename = "/Applications/Firefox.app/Contents/MacOS/firefox"
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}
	t.Skip("Firefox is required for the dynamic Tree production regression")
	return ""
}

func TestTreeDynamicInitializationWithClientAssets(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()
	<-tr.ReadyCh

	// Render the documented target and an initially empty live Container. The Tree
	// itself enters the page only through the subsequent Container update.
	target := ui.NewDiv(template.HTML(`<div id="dynamic"></div>`))
	targetElem := tr.NewElement(target)
	var initial strings.Builder
	if err := targetElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}
	contents := &dynamicTreeContainer{}
	container := ui.NewContainer("div", contents)
	containerElem := tr.NewElement(container)
	if err := containerElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}

	root := &Node{Children: []*Node{{Name: "Documents"}}}
	var mu deadlock.RWMutex
	tree := New("dynamic", ui.NewJsVar(&mu, root), InitiallyExpanded)
	contents.contents = []jaws.UI{tree}
	tr.Dirty(contents)

	frames := make([]wire.WsMsg, 0, 4)
	for len(frames) < 3 {
		select {
		case msg := <-tr.OutCh:
			frames = append(frames, msg)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for dynamic Tree messages; got %+v", frames)
		}
	}
	if frames[0].What != what.Append || frames[0].Jid != containerElem.Jid() {
		t.Fatalf("first dynamic message = %+v, want parent Append", frames[0])
	}
	if frames[1].What != what.Order || frames[1].Jid != containerElem.Jid() {
		t.Fatalf("second dynamic message = %+v, want parent Order", frames[1])
	}
	if frames[2].What != what.Call || frames[2].Jid <= containerElem.Jid() ||
		frames[2].Data != `jawstreeInit={"tree":"dynamic","options":2}` {
		t.Fatalf("third dynamic message = %+v, want child initializer Call", frames[2])
	}
	if strings.Contains(frames[0].Data, "<script") ||
		!strings.Contains(frames[0].Data, `data-jawsname="jawstreeroot_dynamic"`) ||
		!strings.Contains(frames[0].Data, `id="`+frames[2].Jid.String()+`"`) {
		t.Fatalf("dynamic Tree Append has unexpected HTML: %q", frames[0].Data)
	}

	// Queue an ordinary dirty update immediately after insertion. The production
	// writer may coalesce it with the Append/Order/initializer messages above, so
	// the browser regression replays all four in one WebSocket frame.
	tree.Lock()
	root.Children[0].Name = "Updated"
	tree.Unlock()
	jw.Dirty(root)
	select {
	case msg := <-tr.OutCh:
		frames = append(frames, msg)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dynamic Tree update; got %+v", frames)
	}
	if frames[3].What != what.Call || frames[3].Jid != frames[2].Jid ||
		!strings.Contains(frames[3].Data, `"name":"Updated"`) {
		t.Fatalf("fourth dynamic message = %+v, want child jawstreeSet Call", frames[3])
	}

	var dynamicFrame []byte
	for i := range frames {
		dynamicFrame = frames[i].Append(dynamicFrame)
	}
	dynamicFrameJSON, err := json.Marshal(string(dynamicFrame))
	if err != nil {
		t.Fatal(err)
	}
	treeviewJS, err := assetsFS.ReadFile("assets/treeview.js")
	if err != nil {
		t.Fatal(err)
	}
	jawstreeJS, err := assetsFS.ReadFile("assets/jawstree.js")
	if err != nil {
		t.Fatal(err)
	}

	// Recreate the generated defer lifecycle with actual external scripts. The fake
	// WebSocket records whether core JaWS waits for the later deferred dependencies
	// before connecting, then delivers the genuine server frame.
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"jaws.js":     []byte(jawsassets.JavascriptText),
		"jawstree.js": jawstreeJS,
		"treeview.js": treeviewJS,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	htmlText := `<!doctype html><html><head>
<meta name="jawsKey" content="1">
<style>#result{position:fixed;inset:0;z-index:9999;background:rgb(255,0,0)}</style>
<script>
window.dynamicFrame = ` + string(dynamicFrameJSON) + `;
window.WebSocket = class {
	constructor() {
		this.readyState = 1;
		window.connectedAfterAssets =
			typeof window.jawstreeInit === "function" &&
			typeof window.Treeview === "function";
	}
	addEventListener(name, fn) {
		if (name === "message") {
			queueMicrotask(function() {
				fn({data: window.dynamicFrame});
			});
		}
	}
	send() {}
};
</script>
<script defer src="jaws.js"></script>
<script defer src="jawstree.js"></script>
<script defer src="treeview.js"></script>
</head><body>` + initial.String() + `<div id="result"></div>
<script>
window.addEventListener("DOMContentLoaded", function() {
	setTimeout(function() {
		const target = document.getElementById("dynamic");
		const text = target && target.querySelector(".treeview-node-text");
		if (window.connectedAfterAssets &&
			window.jawstree_dynamic instanceof window.Treeview &&
			text && text.textContent === "Updated" && target.querySelector("ul")) {
			document.getElementById("result").style.background = "rgb(0,255,0)";
		}
	}, 0);
});
</script>
</body></html>`

	htmlPath := filepath.Join(dir, "dynamic-tree.html")
	screenshotPath := filepath.Join(dir, "dynamic-tree.png")
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
		t.Fatalf("dynamic Tree did not initialize through shipped assets; center pixel = %#v, want green", pixel)
	}
}
