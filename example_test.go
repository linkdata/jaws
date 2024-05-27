package jaws_test

import (
	"html/template"
	"log"
	"net/http"

	"github.com/linkdata/jaws"
)

const indexhtml = `
<html>
<head>{{$.HeadHTML}}</head>
<body>{{$.Range .Dot}}</body>
</html>
`

func ExampleNew() {
	jw := jaws.New()          // create a default JaWS instance
	defer jw.Close()          // ensure we clean up
	jw.Logger = log.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	jw.AddTemplateLookuper(templates)

	go jw.Serve()                             // start the JaWS processing loop
	http.DefaultServeMux.Handle("/jaws/", jw) // ensure the JaWS routes are handled

	var f jaws.Float // somewhere to store the slider data
	http.DefaultServeMux.Handle("/", jw.Handler("index", &f))
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
