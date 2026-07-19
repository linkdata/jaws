# jawstree

Provides a statically served and embedded version of [Quercus.js](https://github.com/stefaneichert/quercus.js),
a lightweight and customizable JavaScript treeview library with no dependencies.

## Asset provenance

The embedded third-party files are vendored Quercus.js tree assets from
https://github.com/stefaneichert/quercus.js. The `assets/jawstree.*` files are
local adapter source in this repository and are covered by normal code review.

| File | Source |
| --- | --- |
| `assets/treeview.js` | Quercus.js from https://github.com/stefaneichert/quercus.js |
| `assets/treeview.css` | Quercus.js styles from https://github.com/stefaneichert/quercus.js |

When bumping the vendored Quercus files, update this table in the same change.

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
			tree, terr := jawstree.New(&mu, root, jawstree.InitiallyExpanded)
			if terr != nil {
				return terr
			}
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

The examples use `{{$.HeadHTML}}` inside `<head>` to emit the configured
resources and Request key metadata. Applications that provide equivalent markup
may omit it. `{{$.TailHTML}}` is optional; placing it before the closing
`</body>` tag applies updates queued during initial rendering before the
WebSocket connects.

## Using the tree widget

A `Tree` is shared, server-authoritative UI state and the `jaws.UI` that renders
it. Build it once before serving, then render that same `*Tree` for every request
that should show the same collaborative tree; it holds no per-request state, so
sharing it is safe.

`New` takes the lock guarding the tree (which may be shared with other application
state) and the root `*Node`. It returns `ErrInvalidTree` for an invalid graph
(nil, cyclic, shared node, unknown option bit, more than `MaxTreeNodes` nodes,
nesting deeper than `MaxTreeDepth`, or depth-weighted serialized node data
exceeding `MaxTreeRenderBytes`) and `ErrInvalidSelection` when the initial
`Selected` flags violate the mode's policy. It assigns each node's positional-path
ID, a preorder wire index, and the parent back-pointers the name-path API needs, so
it must run before rendering.
Once `New` returns, only the selection may change, through `Tree.SetSelected` or
browser events. Each node's `Name`, `Disabled`, assigned ID, and the topology
(`Children`) are fixed; changing any of them afterward is unsupported, with a different
consequence per field: altering the topology or an ID breaks the ID-to-wire-position
mapping used by Quercus.js; enlarging a `Name` defeats the size bounds `New` enforced
(rendering re-serializes the live tree); toggling `Disabled` can desync the selection
policy.

Build a `Node` tree (by hand, or from a directory with `Root`) and pass it with a
lock to `New`. Browser container IDs are managed internally:

```go
var mu sync.RWMutex
root := &jawstree.Node{Children: []*jawstree.Node{
	{Name: "Documents", Children: []*jawstree.Node{{Name: "report.pdf"}}},
	{Name: "Pictures"},
}}
tree, err := jawstree.New(&mu, root, jawstree.InitiallyExpanded)
if err != nil {
	// handle an invalid tree or initial selection
}
mux.Handle("GET /", ui.Handler(jw, "index.html", tree))
```

In the page template, render the tree directly. Its JaWS-managed `Jid.N` element
is also the Quercus.js container; the browser initializer unhides it after the
deferred page assets are ready. The same initialization works when a JaWS
container or template inserts the Tree through a live DOM update:

```html
<!DOCTYPE html>
<html>
<head>{{$.HeadHTML}}</head>
<body>
  {{$.NewUI .Dot}}
  {{$.TailHTML}}
</body>
</html>
```

Selections made in the browser are validated and applied to the `Node` tree under
the Tree's lock; read them with `Tree.GetSelected` or change them with
`Tree.SetSelected`. After mutating the tree server-side, push the new state to all
rendered clients with `Tree.Dirty`:

```go
_ = tree.SetSelected([][]string{{"Documents", "report.pdf"}})
tree.Dirty(jw)
```
