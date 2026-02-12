package jawstree_test

import (
	"embed"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/jawstree"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/jaws/templatereloader"
	"github.com/linkdata/jaws/ui"
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
	mux.Handle("/jaws/", jw) // Ensure the JaWS routes are handled
	var tmpl jaws.TemplateLookuper
	if tmpl, err = templatereloader.New(assetsFS, "assets/ui/*.html", ""); err == nil {
		jw.AddTemplateLookuper(tmpl)
		// Initialize jawsboot, we will serve the Javascript and CSS from /static/*.[js|css]
		// All files under assets/static will be available under /static. Any favicon loaded
		// this way will have it's URL available using jaws.FaviconURL().
		if err = jw.Setup(mux.Handle, "/static",
			jawsboot.Setup,
			jawstree.Setup,
			staticserve.MustNewFS(assetsFS, "assets/static", "images/favicon.png"),
		); err == nil {
			// Add a route to our index template with a bound variable accessible as '.Dot' in the template
			var mu sync.Mutex
			var f float64
			mux.Handle("/", ui.Handler(jw, "index.html", jaws.Bind(&mu, &f)))
		}
	}
	return
}

func Example() {
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
