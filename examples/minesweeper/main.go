package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/ui"
)

//go:embed assets/ui/*.html assets/static/*.css
var assetsFS embed.FS

// run configures and serves the Minesweeper application.
//
// The listener is injected so tests can exercise the complete HTTP handler
// without opening a network port.
func run(listenAndServe func(addr string, handler http.Handler) error) (err error) {
	const addr = ":8080"

	var jw *jaws.Jaws
	if jw, err = jaws.New(); err == nil {
		jw.Logger = slog.Default()
		defer jw.Close()

		var tmpl *template.Template
		if tmpl, err = template.ParseFS(assetsFS, "assets/ui/*.html"); err == nil {
			if err = jw.AddTemplateLookuper(tmpl); err == nil {
				if err = jw.GenerateHeadHTML("/static/style.css"); err == nil {
					var staticFiles fs.FS
					if staticFiles, err = fs.Sub(assetsFS, "assets/static"); err == nil {
						go jw.Serve()

						// ui.Handler reuses its Dot across Requests. Keeping the game here
						// deliberately gives every connected browser the same synchronized board.
						sharedGame := newGame(10, 10, 15)
						page := jw.SecureHeadersMiddleware(ui.Handler(jw, "index.html", sharedGame))

						mux := http.NewServeMux()
						mux.Handle("GET /jaws/", jw)
						mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
						mux.Handle("GET /", page)

						slog.Info("Minesweeper is listening", "url", "http://localhost:8080")
						err = listenAndServe(addr, mux)
					}
				}
			}
		}
	}
	return
}

func main() {
	if err := run(http.ListenAndServe); err != nil {
		slog.Error("Minesweeper stopped", "error", err)
		os.Exit(1)
	}
}
