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
	"strconv"
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

func TestTreeInitialAndDynamicInitializationWithClientAssets(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("nil test request")
	}
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()
	<-tr.ReadyCh

	// Render an initial Tree and its documented target before the WebSocket queue
	// is flushed. Its Call will exercise dependency readiness in the browser.
	initialTarget := ui.NewDiv(template.HTML(`<div id="initial"></div>`))
	initialTargetElem := tr.NewElement(initialTarget)
	var initial strings.Builder
	if err := initialTargetElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}
	initialRoot := &Node{Children: []*Node{{Name: "Before assets"}}}
	var initialMu deadlock.RWMutex
	initialTree := New("initial", ui.NewJsVar(&initialMu, initialRoot))
	initialTreeElem := tr.NewElement(initialTree)
	if err := initialTreeElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}

	// Render the dynamic Tree's documented target through the normal UI path,
	// then render the initially empty dynamic Container alongside it.
	target := ui.NewDiv(template.HTML(`<div id="dynamic"></div>`))
	targetElem := tr.NewElement(target)
	if err := targetElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}
	contents := &dynamicTreeContainer{}
	container := ui.NewContainer("div", contents)
	containerElem := tr.NewElement(container)
	if err := containerElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}

	// Render a second target and empty live Container for the remove-before-ready
	// lifecycle. Its Tree will be appended and removed entirely through updates.
	staleTarget := ui.NewDiv(template.HTML(`<div id="stale"></div>`))
	staleTargetElem := tr.NewElement(staleTarget)
	if err := staleTargetElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}
	staleContents := &dynamicTreeContainer{}
	staleContainer := ui.NewContainer("div", staleContents)
	staleContainerElem := tr.NewElement(staleContainer)
	if err := staleContainerElem.JawsRender(&initial, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(initial.String(), `<div id="initial"></div>`) ||
		!strings.Contains(initial.String(), `<div id="dynamic"></div>`) ||
		!strings.Contains(initial.String(), `<div id="stale"></div>`) {
		t.Fatalf("initial page is missing a documented Tree target: %q", initial.String())
	}

	tr.InCh <- wire.WsMsg{}
	var initialCall wire.WsMsg
	select {
	case initialCall = <-tr.OutCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial Tree initializer")
	}
	if initialCall.What != what.Call || initialCall.Jid != initialTreeElem.Jid() {
		t.Fatalf("initial Tree message = %+v, want element-scoped Call for %s", initialCall, initialTreeElem.Jid())
	}
	if want := `jawsCallWhenReady={"id":` + strconv.Quote(initialTreeElem.Jid().String()) + `,"path":"jawstreeInit","data":{"tree":"initial","options":0}}`; initialCall.Data != want {
		t.Fatalf("initial Tree data = %q, want %q", initialCall.Data, want)
	}

	root := &Node{Children: []*Node{{Name: "Documents"}}}
	var mu deadlock.RWMutex
	tree := New("dynamic", ui.NewJsVar(&mu, root), InitiallyExpanded)
	contents.contents = []jaws.UI{tree}

	// This is the supported production path: dirtying the Container's source lets
	// the Request process loop render the new Tree and send its DOM and Call frames.
	tr.Dirty(contents)

	got := make([]wire.WsMsg, 0, 3)
	for len(got) < 3 {
		select {
		case msg := <-tr.OutCh:
			got = append(got, msg)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for dynamic tree messages; got %+v", got)
		}
	}

	if got[0].What != what.Append || got[0].Jid != containerElem.Jid() {
		t.Fatalf("first dynamic message = %+v, want parent Append", got[0])
	}
	if strings.Contains(got[0].Data, "<script") {
		t.Fatalf("dynamic Tree HTML contains an inert script: %q", got[0].Data)
	}
	if !strings.Contains(got[0].Data, `data-jawsname="jawstreeroot_dynamic"`) {
		t.Fatalf("dynamic Tree HTML is missing root data: %q", got[0].Data)
	}
	if got[1].What != what.Order || got[1].Jid != containerElem.Jid() {
		t.Fatalf("second dynamic message = %+v, want parent Order", got[1])
	}
	if got[2].What != what.Call || got[2].Jid <= containerElem.Jid() {
		t.Fatalf("third dynamic message = %+v, want child initializer Call", got[2])
	}
	if !strings.Contains(got[0].Data, `id="`+got[2].Jid.String()+`"`) {
		t.Fatalf("Append HTML does not contain initializer target %s: %q", got[2].Jid, got[0].Data)
	}
	if want := `jawsCallWhenReady={"id":` + strconv.Quote(got[2].Jid.String()) + `,"path":"jawstreeInit","data":{"tree":"dynamic","options":2}}`; got[2].Data != want {
		t.Fatalf("initializer data = %q, want %q", got[2].Data, want)
	}

	staleRoot := &Node{Children: []*Node{{Name: "Must not appear"}}}
	var staleMu deadlock.RWMutex
	staleTree := New("stale", ui.NewJsVar(&staleMu, staleRoot))
	staleContents.contents = []jaws.UI{staleTree}
	tr.Dirty(staleContents)

	staleAdd := make([]wire.WsMsg, 0, 3)
	for len(staleAdd) < 3 {
		select {
		case msg := <-tr.OutCh:
			staleAdd = append(staleAdd, msg)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for stale Tree append messages; got %+v", staleAdd)
		}
	}
	if staleAdd[0].What != what.Append || staleAdd[0].Jid != staleContainerElem.Jid() {
		t.Fatalf("first stale Tree message = %+v, want parent Append", staleAdd[0])
	}
	if strings.Contains(staleAdd[0].Data, "<script") ||
		!strings.Contains(staleAdd[0].Data, `data-jawsname="jawstreeroot_stale"`) {
		t.Fatalf("stale Tree Append has unexpected HTML: %q", staleAdd[0].Data)
	}
	if staleAdd[1].What != what.Order || staleAdd[1].Jid != staleContainerElem.Jid() {
		t.Fatalf("second stale Tree message = %+v, want parent Order", staleAdd[1])
	}
	if staleAdd[2].What != what.Call || staleAdd[2].Jid <= staleContainerElem.Jid() {
		t.Fatalf("third stale Tree message = %+v, want child initializer Call", staleAdd[2])
	}
	if !strings.Contains(staleAdd[0].Data, `id="`+staleAdd[2].Jid.String()+`"`) {
		t.Fatalf("stale Append HTML does not contain initializer target %s: %q", staleAdd[2].Jid, staleAdd[0].Data)
	}
	if want := `jawsCallWhenReady={"id":` + strconv.Quote(staleAdd[2].Jid.String()) + `,"path":"jawstreeInit","data":{"tree":"stale","options":0}}`; staleAdd[2].Data != want {
		t.Fatalf("stale initializer data = %q, want %q", staleAdd[2].Data, want)
	}

	// Remove the Tree before browser readiness. The queued Call remains in the
	// already-sent frame, so the client must use its originating Jid to suppress it.
	staleContents.contents = nil
	tr.Dirty(staleContents)
	var staleRemove wire.WsMsg
	select {
	case staleRemove = <-tr.OutCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stale Tree removal")
	}
	if staleRemove.What != what.Remove || staleRemove.Jid != staleContainerElem.Jid() ||
		staleRemove.Data != staleAdd[2].Jid.String() {
		t.Fatalf("stale Tree removal = %+v, want parent Remove of %s", staleRemove, staleAdd[2].Jid)
	}

	// Replay the initial Call after jaws.js has loaded but before jawstree.js and
	// treeview.js. The readiness queue must retain it until both dependencies exist.
	initialFrameJSON, err := json.Marshal(string(initialCall.Append(nil)))
	if err != nil {
		t.Fatal(err)
	}
	// Replay the live Container frames after DOMContentLoaded, matching the normal
	// dynamic-update path after all shipped assets are ready.
	var dynamicFrame []byte
	for i := range got {
		dynamicFrame = got[i].Append(dynamicFrame)
	}
	dynamicFrameJSON, err := json.Marshal(string(dynamicFrame))
	if err != nil {
		t.Fatal(err)
	}
	// Replay both update passes before readiness: the Append queues initialization,
	// then the Remove deletes its originating element while the static target stays.
	var staleLifecycleFrame []byte
	for i := range staleAdd {
		staleLifecycleFrame = staleAdd[i].Append(staleLifecycleFrame)
	}
	staleLifecycleFrame = staleRemove.Append(staleLifecycleFrame)
	staleLifecycleFrameJSON, err := json.Marshal(string(staleLifecycleFrame))
	if err != nil {
		t.Fatal(err)
	}
	jawstreeJS, err := assetsFS.ReadFile("assets/jawstree.js")
	if err != nil {
		t.Fatal(err)
	}
	treeviewJS, err := assetsFS.ReadFile("assets/treeview.js")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "dynamic-tree.html")
	screenshotPath := filepath.Join(dir, "dynamic-tree.png")
	profilePath := filepath.Join(dir, "firefox-profile")
	if err := os.Mkdir(profilePath, 0o700); err != nil {
		t.Fatal(err)
	}
	htmlText := `<!doctype html><html><head>
<meta name="jawsKey" content="1">
<style>#result{position:fixed;inset:0;z-index:9999;background:rgb(255,0,0)}</style>
</head><body>` + initial.String() + `<div id="result"></div>
<script>
window.WebSocket = class {
	constructor() { this.readyState = 1; }
	addEventListener() {}
	send() {}
};
</script>
<script>` + jawsassets.JavascriptText + `</script>
<script>
jawsMessage({data:` + string(initialFrameJSON) + `});
jawsMessage({data:` + string(staleLifecycleFrameJSON) + `});
window.jawstreeInitializerWasPending = (window.jawstree_initial === undefined);
window.jawstreeRemovedBeforeReady =
	(document.getElementById(` + strconv.Quote(staleAdd[2].Jid.String()) + `) === null);
</script>
<script>` + string(jawstreeJS) + `</script>
<script>` + string(treeviewJS) + `</script>
<script>
window.addEventListener("DOMContentLoaded", function() {
	const initialTarget = document.getElementById("initial");
	const initialText = initialTarget && initialTarget.querySelector(".treeview-node-text");
	const initialTree = window.jawstree_initial;
	jawsMessage({data:` + string(dynamicFrameJSON) + `});
	const dynamicTarget = document.getElementById("dynamic");
	const dynamicText = dynamicTarget && dynamicTarget.querySelector(".treeview-node-text");
	const dynamicTree = window.jawstree_dynamic;
	const staleTarget = document.getElementById("stale");
	if (window.jawstreeInitializerWasPending &&
		window.jawstreeRemovedBeforeReady &&
		initialTree instanceof window.Treeview &&
		initialText && initialText.textContent === "Before assets" &&
		initialTarget.querySelector("ul") &&
		dynamicTree instanceof window.Treeview &&
		dynamicText && dynamicText.textContent === "Documents" &&
		dynamicTarget.querySelector("ul") &&
		window.jawstree_stale === undefined &&
		staleTarget && !staleTarget.querySelector("ul")) {
		document.getElementById("result").style.background = "rgb(0,255,0)";
	}
});
</script>
</body></html>`
	if err := os.WriteFile(htmlPath, []byte(htmlText), 0o600); err != nil {
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
		t.Fatalf("Tree readiness lifecycle failed through the shipped client assets; center pixel = %#v, want green", pixel)
	}
}
