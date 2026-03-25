package jawstest

import (
	"html/template"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

const bindsuccess_templ = `
{{with .Dot}}
	{{$.Span .Setter }}
	<span class="form-control">
		{{$.Range .Setter}}
	</span>
	{{$.Button "Reset" .}}
{{end}}
`

type bindSuccessTester struct {
	jaws.Setter[int]
}

func newBindSuccessTester() bindSuccessTester {
	var mu sync.RWMutex
	var val int
	ui := bindSuccessTester{
		Setter: jaws.Bind(&mu, &val), //.Success(func() {}),
	}
	return ui
}

func (ui bindSuccessTester) JawsClick(e *jaws.Element, name string) (err error) {
	err = jaws.ErrEventUnhandled
	return
}

func TestBindSuccess(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	bst := newBindSuccessTester()
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("bindsuccess").Parse(bindsuccess_templ)))
	err := rq.Template("bindsuccess", bst)
	if err != nil {
		t.Fatal(err)
	}
}
