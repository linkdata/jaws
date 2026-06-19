# jawstree

Provides a statically served and embedded version of [Quercus.js](https://github.com/stefaneichert/quercus.js),
a lightweight and customizable JavaScript treeview library with no dependencies.

```go
package main

import (
	"embed"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/jawstree"
	"github.com/linkdata/jaws/lib/templatereloader"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/staticserve"
)

// This example assumes an 'assets' directory:
//
//.  assets/
//.    static/
//.      images/
//.        favicon.png
//.    ui/
//.      index.html

//go:embed assets
var assetsFS embed.FS

func setupJaws(jw *jaws.Jaws, mux *http.ServeMux) (err error) {
	mux.Handle("GET /jaws/", jw) // Ensure the JaWS routes are handled
	var tmpl jaws.TemplateLookuper
	if tmpl, err = templatereloader.New(assetsFS, "assets/ui/*.html", ""); err == nil {
		jw.AddTemplateLookuper(tmpl)
		// Initialize jawsboot; we will serve the JavaScript and CSS from /static/*.[js|css].
		// All files under assets/static will be available under /static. Any favicon loaded
		// this way will have its URL available using jw.FaviconURL().
		if err = jw.Setup(mux.Handle, "/static",
			jawsboot.Setup,
			jawstree.Setup,
			staticserve.MustNewFS(assetsFS, "assets/static", "images/favicon.png"),
		); err == nil {
			var mu sync.RWMutex
			root := &jawstree.Node{Children: []*jawstree.Node{
				{Name: "Documents", Children: []*jawstree.Node{{Name: "report.pdf"}}},
				{Name: "Pictures"},
			}}
			tree := jawstree.New("mytree", ui.NewJsVar(&mu, root), jawstree.InitiallyExpanded)
			mux.Handle("GET /", ui.Handler(jw, "index.html", tree))
		}
	}
	return
}

func main() {
	jw, err := jaws.New()
	if err == nil {
		jw.Logger = slog.Default()
		if err = setupJaws(jw, http.DefaultServeMux); err == nil {
			// start the JaWS processing loop and the HTTP server
			go jw.Serve()
			slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
		}
	}
	if err != nil {
		panic(err)
	}
}
```

The example expects an `assets` directory in the source tree:

```
assets
├── static
│   └── images
│       └── favicon.png
└── ui
    └── index.html
```

Page templates rendered through `ui.Handler` should include `{{$.HeadHTML}}`
inside `<head>` and `{{$.TailHTML}}` before the closing `</body>` tag.

## Using the tree widget

A `Tree` is shared UI state. Build it once before serving or rendering it, then
reuse that `*Tree` for every request that should show the same tree. The embedded
`ui.JsVar` is the backing store, lock, and browser communication channel for the
`Node` tree.

Build a `Node` tree (by hand, or from a directory with `Root`), wrap its root
in a `ui.JsVar`, and pass it to `New`. `New` initializes node IDs plus the tree
and parent back-pointers, so it must run before rendering or using the name-path
selection API:

```go
var mu sync.RWMutex
root := &jawstree.Node{Children: []*jawstree.Node{
	{Name: "Documents", Children: []*jawstree.Node{{Name: "report.pdf"}}},
	{Name: "Pictures"},
}}
tree := jawstree.New("mytree", ui.NewJsVar(&mu, root), jawstree.InitiallyExpanded)
mux.Handle("GET /", ui.Handler(jw, "index.html", tree))
```

In the page template, render the tree (it emits a hidden data element and the
init script) and provide a container element whose HTML id equals the tree id;
Quercus.js renders the tree into that container, and without it the tree
silently fails to appear:

```html
<!DOCTYPE html>
<html>
<head>{{$.HeadHTML}}</head>
<body>
  {{$.NewUI .Dot}}
  <div id="mytree"></div>
  {{$.TailHTML}}
</body>
</html>
```

Selections made in the browser are applied to the `Node` tree under the
`ui.JsVar` lock; read them with `Tree.GetSelected` or change them with
`Tree.SetSelected`. After mutating the tree server-side, push the new state to
all rendered clients by dirtying the JsVar's bound pointer:

```go
tree.SetSelected([][]string{{"Documents", "report.pdf"}})
jw.Dirty(root)
```
