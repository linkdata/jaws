package example

import (
	"html/template"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/ui"
)

const indexhtml = `
<html>
  <head>{{$.HeadHTML}}</head>
  <body>{{with .Dot}}
    {{$.Range .}}
  {{end}}{{$.TailHTML}}</body>
</html>
`

// Example is a compile-checked illustration of a minimal JaWS application. It
// starts a blocking HTTP server, so it has no testable Output and is not
// executed by "go test".
func Example() {
	jw, err := jaws.New() // create a default JaWS instance
	if err != nil {
		panic(err)
	}
	defer jw.Close()           // ensure we clean up
	jw.Logger = slog.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	if err := jw.AddTemplateLookuper(templates); err != nil {
		panic(err)
	}

	go jw.Serve()                                 // start the JaWS processing loop
	http.DefaultServeMux.Handle("GET /jaws/", jw) // ensure the JaWS routes are handled

	var mu sync.Mutex
	var f float64

	http.DefaultServeMux.Handle("GET /", ui.Handler(jw, "index", bind.New(&mu, &f)))
	slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
}

// Example_secureSession is a compile-checked illustration of adding sessions and
// secure headers. Like Example it starts a blocking server, so it has no
// testable Output and is not executed by "go test".
func Example_secureSession() {
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	defer jw.Close()
	jw.Logger = slog.Default()

	templates := template.Must(template.New("index").Parse(indexhtml))
	if err := jw.AddTemplateLookuper(templates); err != nil {
		panic(err)
	}

	go jw.Serve()
	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)

	var mu sync.Mutex
	var f float64

	page := ui.Handler(jw, "index", bind.New(&mu, &f))
	mux.Handle("GET /", jw.SessionMiddleware(jw.SecureHeadersMiddleware(page)))
	slog.Error(http.ListenAndServe("localhost:8080", mux).Error())
}
