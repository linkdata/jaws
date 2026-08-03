package ui

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// benchOwnedTemplates renders a wrapped template containing many unwrapped nested
// templates, the shape that makes a template update replace a whole generation of
// Elements at once.
const benchOwnedTemplates = `
{{define "bench-parent"}}{{range $.Dot.Names}}{{$.RequestWriter.Template "" "bench-leaf" $.Dot}}{{end}}{{end}}
{{define "bench-leaf"}}leaf{{end}}
`

// benchOwnedDot is the dot for the benchmark templates. A pointer is usable as a tag.
type benchOwnedDot struct{ names []string }

func (d *benchOwnedDot) Names() []string { return d.names }

// benchOwnedFixture builds a rendered wrapped template with nested Elements and
// returns the Jaws to close plus a func performing one update. Each call gets its own
// Jaws and Request, so an implementation that leaks Elements cannot accumulate them
// across benchmark iterations and skew the result.
//
// The update is returned as a closure rather than the template and Element, so this
// file also compiles against a revision where NewTemplate returns a value: only type
// inference sees the difference.
func benchOwnedFixture(b *testing.B, nested int) (jw *jaws.Jaws, update func()) {
	b.Helper()
	var err error
	if jw, err = jaws.New(); err != nil {
		b.Fatal(err)
	}
	if err = jw.AddTemplateLookuper(template.Must(template.New("bench").Parse(benchOwnedTemplates))); err != nil {
		jw.Close()
		b.Fatal(err)
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		jw.Close()
		b.Fatal("nil request")
	}
	names := make([]string, nested)
	for i := range names {
		names[i] = "row-" + strconv.Itoa(i)
	}
	tmpl := NewTemplate("div", "bench-parent", &benchOwnedDot{names: names})
	elem := rq.NewElement(tmpl)
	// Extra tags on the wrapper, so unregistering a generation has to visit more than
	// one tag entry: removeElementsLocked scans every tag entry per call.
	if err = elem.JawsRender(io.Discard, []any{tag.Tag("alpha"), tag.Tag("beta"), tag.Tag("gamma")}); err != nil {
		jw.Close()
		b.Fatal(err)
	}
	update = func() { tmpl.JawsUpdate(elem) }
	return
}

// BenchmarkTemplateUpdateOwnedCleanup measures one update of a wrapped template with
// many nested Elements: it renders a fresh generation and unregisters the replaced
// one. It guards the batched unregister against regressing to a per-element
// registry pass.
func BenchmarkTemplateUpdateOwnedCleanup(b *testing.B) {
	b.ReportAllocs()
	const nested = 1000
	for range b.N {
		b.StopTimer()
		jw, update := benchOwnedFixture(b, nested)
		b.StartTimer()
		update()
		b.StopTimer()
		jw.Close()
	}
}
