package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
)

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
	rq := jw.NewRequest(nil)
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

type exampleContainer []string

func (c exampleContainer) JawsContains(elem *jaws.Element) (contents []jaws.UI) {
	for _, item := range c {
		contents = append(contents, ui.NewSpan(item))
	}
	return
}

func ExampleContainerHelper_renderScoped() {
	first := ui.NewContainer("div", exampleContainer{"one"})
	second := ui.NewContainer("div", exampleContainer{"two"})
	fmt.Println(first == second)

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	_ = enc.Encode([]string{"construct containers during render"})
	fmt.Print(b.String())

	// Output:
	// false
	// ["construct containers during render"]
}
