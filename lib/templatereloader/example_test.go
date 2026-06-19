package templatereloader_test

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws/lib/templatereloader"
)

func ExampleTemplateReloader_LastError() {
	tmpl := template.Must(template.New("index.html").Parse("ok"))
	lookuper, ok := any(tmpl).(interface {
		Lookup(string) *template.Template
	})
	if !ok {
		panic("template does not implement Lookup")
	}
	fmt.Println(lookuper.Lookup("index.html") != nil)

	var reloader *templatereloader.TemplateReloader
	fmt.Println(reloader.LastError())

	// Output:
	// true
	// <nil>
}
