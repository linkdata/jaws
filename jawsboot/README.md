# jawsboot

Provides a statically served and embedded version of [Bootstrap](https://getbootstrap.com/) (v5.3.8).

## Asset provenance

The embedded files are vendored from Bootstrap v5.3.8 and stored gzip-compressed
under `assets/static`. The "gzipped blob" hash verifies the committed `.gz` file;
the "uncompressed" hash matches a direct download of the artifact from
https://getbootstrap.com/ (compare with `gzip -dc file.gz | sha256sum`).

| File | Upstream artifact | SHA-256 (gzipped blob) | SHA-256 (uncompressed) |
| --- | --- | --- | --- |
| `assets/static/bootstrap.bundle.min.js.gz` | `bootstrap.bundle.min.js` | `4d0ae6252e765ecd243be3904526dd15605f14d100a38ba438622c5cb0de06c7` | `e4fd49181388c48ec5040bd3fe66f57c29c8e67fcd8502b3354b96ec7ab47cc7` |
| `assets/static/bootstrap.min.css.gz` | `bootstrap.min.css` | `1ad6a4582202314026750ae78bdb719b096ebf3fd9bf018df081ed5e41234595` | `d85327d99c7a3ee1f9b5d0500d1370acea3ad2db39c163c2f51f232baedbdede` |

When bumping Bootstrap, update this table and `doc.go` in the same change.

Example usage that loads your templates, favicon and Bootstrap. Also uses a `templatereloader`
so that when running with `-tags debug` or `-race` templates are reloaded from disk as needed.

```go
package main

import (
	"embed"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/templatereloader"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/staticserve"
)

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
			staticserve.MustNewFS(assetsFS, "assets/static", "images/favicon.png"),
		); err == nil {
			// Add a route to our index template with a bound variable accessible as '.Dot' in the template
			var mu sync.Mutex
			var f float64
			mux.Handle("GET /", ui.Handler(jw, "index.html", bind.New(&mu, &f)))
		}
	}
	return
}

func main() {
	if jw, err := jaws.New(); err == nil {
		jw.Logger = slog.Default()
		if err = setupJaws(jw, http.DefaultServeMux); err == nil {
			// start the JaWS processing loop and the HTTP server
			go jw.Serve()
			slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
		}
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
    ├── somepage.html
    ├── otherpage.html
    └── index.html
```

Page templates rendered through `ui.Handler` should include `{{$.HeadHTML}}`
inside `<head>` and `{{$.TailHTML}}` before the closing `</body>` tag.
