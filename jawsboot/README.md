# jawsboot

Provides a statically served and embedded version of [Bootstrap](https://getbootstrap.com/).

Example usage that loads your templates, favicon and Bootstrap. Also uses a `templatereloader`
so that when running with `-tags debug` or `-race` templates are reloaded from disk as needed.

```go
//go:embed assets
var assetsFS embed.FS

func setupRoutes(jw *jaws.Jaws, mux *http.ServeMux) (err error) {
	var tmpl jaws.TemplateLookuper
	if tmpl, err = templatereloader.New(assetsFS, "assets/ui/*.html", ""); err == nil {
		jw.AddTemplateLookuper(tmpl)
		var faviconuri string
		if faviconuri, err = staticserve.HandleFS(assetsFS, "assets", "static/images/favicon.png", mux.Handle); err == nil {
			if err = jawsboot.Setup(jw, mux.Handle, faviconuri); err == nil {
				// set up your other routes
			}
		}
	}
	return
}
```

Expects an `assets` directory in the source tree:

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
