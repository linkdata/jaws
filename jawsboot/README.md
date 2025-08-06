# jawsboot

Provides a statically served and embedded version of [Bootstrap](https://getbootstrap.com/).

Example usage that loads your templates, favicon and Bootstrap. Also uses a `templatereloader`
so that when running with `-tags debug` or `-race` templates are reloaded from disk as needed.

```go
//go:embed assets
var assetsFS embed.FS

func setupRoutes(jw *jaws.Jaws, mux *http.ServeMux) (faviconuri string, err error) {
	var tmpl jaws.TemplateLookuper
	if tmpl, err = templatereloader.New(assetsFS, "assets/ui/*.html", ""); err == nil {
		jw.AddTemplateLookuper(tmpl)
		if uris, err = staticserve.HandleFS(assetsFS, mux.Handle, "assets", "static/images/favicon.png"); err == nil {
			if err = jawsboot.Setup(jw, mux.Handle, uris...); err == nil {
				mux.Handle("/jaws/", jw) // ensure the JaWS routes are handled
				// set up your other routes
			}
		}
	}
	return
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
