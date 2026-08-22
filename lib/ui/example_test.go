package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
)

const exampleConnectionsHTML = `<html>
<head>{{$.HeadHTML}}</head>
<body>{{$.Span .Dot.Count}}{{$.TailHTML}}</body>
</html>`

type exampleConnections struct {
	mu    sync.RWMutex
	count int
}

// Count returns the accepted connection count as a direct field binding.
func (state *exampleConnections) Count() bind.Binder[int] {
	return bind.New(&state.mu, &state.count)
}

// JawsConnect records an accepted JaWS client connection.
func (state *exampleConnections) JawsConnect(rq *jaws.Request) error {
	state.mu.Lock()
	state.count++
	state.mu.Unlock()
	rq.Dirty(&state.count)
	return nil
}

var _ jaws.ConnectHandler = (*exampleConnections)(nil)

func ExampleHandler_connectHandler() {
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	defer jw.Close()
	jw.Logger = slog.Default()

	templates := template.Must(template.New("connections").Parse(exampleConnectionsHTML))
	if err = jw.AddTemplateLookuper(templates); err != nil {
		panic(err)
	}

	go jw.Serve()
	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)
	mux.Handle("GET /", ui.Handler(jw, "connections", new(exampleConnections)))

	_ = mux // serve mux with an HTTP server
}

type examplePathState struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

func (state *examplePathState) JawsSetPath(elem *jaws.Element, jsPath string, value any) error {
	if jsPath != "title" {
		return fmt.Errorf("%w: %s", ui.ErrIllegalJsVarPath, jsPath)
	}
	title, ok := value.(string)
	if !ok {
		return fmt.Errorf("title: %T", value)
	}
	if state.Title == title {
		return jaws.ErrValueUnchanged
	}
	state.Title = title
	return nil
}

func ExampleJsVar_pathSetter() {
	var mu sync.Mutex
	state := examplePathState{Title: "old", Items: []string{"server-owned"}}
	jsv := ui.NewJsVar(&mu, &state)

	if err := jsv.JawsSetPath(nil, "title", "new"); err != nil {
		panic(err)
	}
	err := jsv.JawsSetPath(nil, "items.1", "blocked")
	fmt.Println(state.Title)
	fmt.Println(errors.Is(err, ui.ErrIllegalJsVarPath))

	// Output:
	// new
	// true
}

func ExampleJSONSizeCheck() {
	type clientState struct {
		Items []string `json:"items"`
	}

	var mu sync.Mutex
	state := clientState{}
	jsv := ui.NewJsVar(&mu, &state)
	jsv.ClientCheck = ui.JSONSizeCheck[clientState](1 << 20)

	_ = jsv // render this request-scoped binding normally
}

func ExampleTemplate_failureBehavior() {
	tmpl := template.Must(template.New("partial").Parse(`before {{.Dot}} {{call .Missing}} after`))
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	defer jw.Close()
	if err = jw.AddTemplateLookuper(tmpl); err != nil {
		panic(err)
	}
	rq := jw.NewRequest(httptest.NewRecorder(), nil)
	elem := rq.NewElement(ui.NewTemplate("div", "partial", tag.Tag("dot")))

	var out bytes.Buffer
	err = elem.JawsRender(&out, nil)
	fmt.Println(strings.Contains(out.String(), `id="Jid.1"`))
	fmt.Println(strings.Contains(out.String(), "before dot"))
	fmt.Println(err != nil)

	// Output:
	// true
	// true
	// true
}

func ExampleNewTemplate_defaultWrapper() {
	tmpl := ui.NewTemplate("", "partial", tag.Tag("dot"))
	fmt.Println(tmpl.OuterHTMLTag)

	// Output:
	// div
}

type exampleContainer []string

func (c exampleContainer) JawsContains(elem *jaws.Element) (contents []jaws.UI) {
	for _, item := range c {
		contents = append(contents, ui.NewSpan(item))
	}
	return
}

func ExampleContainer_renderScoped() {
	firstRows := exampleContainer{"one"}
	secondRows := exampleContainer{"two"}
	first := ui.NewContainer("div", &firstRows)
	second := ui.NewContainer("div", &secondRows)
	fmt.Println(first == second)

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	_ = enc.Encode([]string{"construct containers during render"})
	fmt.Print(b.String())

	// Output:
	// false
	// ["construct containers during render"]
}
