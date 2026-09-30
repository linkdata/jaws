# jawsboot

Provides a statically served and embedded version of [Bootstrap](https://getbootstrap.com/) (v5.3.8).

## Asset provenance

The embedded files are vendored from Bootstrap v5.3.8 (downloaded from
https://getbootstrap.com/) and stored gzip-compressed under `assets/static`:

| File | Upstream artifact |
| --- | --- |
| `assets/static/bootstrap.bundle.min.js.gz` | `bootstrap.bundle.min.js` |
| `assets/static/bootstrap.min.css.gz` | `bootstrap.min.css` |

Maintainers should follow the [Bootstrap version update checklist](./AI.md#bootstrap-version-update-checklist).

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

func setupJaws(jw *jaws.Jaws, mux *http.ServeMux) error {
	mux.Handle("GET /jaws/", jw) // Ensure the JaWS routes are handled
	tmpl, err := templatereloader.New(assetsFS, "assets/ui/*.html", "")
	if err != nil {
		return err
	}
	if err := jw.AddTemplateLookuper(tmpl); err != nil {
		return err
	}
	// Initialize jawsboot; we will serve the JavaScript and CSS from /static/*.[js|css].
	// All files under assets/static will be available under /static. Any favicon loaded
	// this way will have its URL available using jw.FaviconURL().
	if err := jw.Setup(
		mux.Handle, "/static",
		jawsboot.Setup,
		staticserve.MustNewFS(assetsFS, "assets/static", "images/favicon.png"),
	); err != nil {
		return err
	}
	// Add a route to our index template with a bound variable accessible as '.Dot' in the template
	var mu sync.Mutex
	var f float64
	mux.Handle("GET /{$}", ui.Handler(jw, "index.html", bind.New(&mu, &f)))
	return nil
}

func main() {
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	defer jw.Close()
	jw.Logger = slog.Default()
	if err := setupJaws(jw, http.DefaultServeMux); err != nil {
		panic(err)
	}
	// start the JaWS processing loop and the HTTP server
	go jw.Serve()
	panic(http.ListenAndServe("localhost:8080", nil))
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

The examples use `{{$.HeadHTML}}` inside `<head>` to emit the configured
resources and Request key metadata. Applications that provide equivalent markup
may omit it. `{{$.TailHTML}}` is optional; placing it before the closing
`</body>` tag applies updates queued during initial rendering before the
WebSocket connects.
