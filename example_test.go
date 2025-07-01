package jaws_test

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/linkdata/jaws"
)

const indexhtml = `
<html>
  <head>{{$.HeadHTML}}</head>
  <body>{{with .Dot}}
    {{$.Range .}}
    {{$.TailHTML}}
  {{end}}</body>
</html>
`

func Example() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	jw, err := jaws.New() // create a default JaWS instance
	if err != nil {
		panic(err)
	}
	defer jw.Close()           // ensure we clean up
	jw.Logger = slog.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	jw.AddTemplateLookuper(templates)

	go jw.Serve(ctx)                          // start the JaWS processing loop
	http.DefaultServeMux.Handle("/jaws/", jw) // ensure the JaWS routes are handled

	var mu sync.Mutex
	var f float64

	http.DefaultServeMux.Handle("/", jw.Handler("index", jaws.Bind(&mu, &f)))
	slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
}
