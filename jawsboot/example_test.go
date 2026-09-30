package jawsboot_test

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

// Example wires jawsboot into an HTTP server. It is a compile-checked
// illustration only: it starts a blocking server, so it has no testable Output
// and is not executed by "go test".
func Example() {
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
