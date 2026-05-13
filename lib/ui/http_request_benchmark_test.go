package ui

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

const benchmarkRemoteAddr = "192.0.2.1:12345"

func BenchmarkHTTPPageRenderingByComplexity(b *testing.B) {
	cases := []struct {
		name    string
		handler func(*testing.B, *jaws.Jaws) http.Handler
	}{
		{name: "Complexity1_StaticHTML", handler: newBenchmarkStaticHTMLHandler},
		{name: "Complexity2_PageTemplate", handler: newBenchmarkPageTemplateHandler},
		{name: "Complexity3_SimpleWidgets", handler: newBenchmarkSimpleWidgetsHandler},
		{name: "Complexity4_RepeatedWidgets", handler: newBenchmarkRepeatedWidgetsHandler},
		{name: "Complexity5_InputsAndTemplates", handler: newBenchmarkInputsAndTemplatesHandler},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			jw, err := jaws.New()
			if err != nil {
				b.Fatal(err)
			}
			defer jw.Close()

			handler := bc.handler(b, jw)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := benchmarkPageRequest()
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)
				if got := benchmarkStatusCode(rr); got != http.StatusOK {
					b.Fatalf("page status = %d, want %d", got, http.StatusOK)
				}

				b.StopTimer()
				benchmarkCleanupNoscript(b, jw, req.RemoteAddr, rr.Body.String())
				b.StartTimer()
			}
			b.StopTimer()

			if got := jw.RequestCount(); got != 0 {
				b.Fatalf("request count after benchmark = %d, want 0", got)
			}
		})
	}
}

type benchmarkStaticHTMLHandler struct {
	jw *jaws.Jaws
}

func newBenchmarkStaticHTMLHandler(_ *testing.B, jw *jaws.Jaws) http.Handler {
	return benchmarkStaticHTMLHandler{jw: jw}
}

func (h benchmarkStaticHTMLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rq := h.jw.NewRequest(r)
	_, _ = io.WriteString(w, "<!doctype html><html><head><title>Static</title>")
	_ = rq.HeadHTML(w)
	_, _ = io.WriteString(w, `</head><body><main><h1>Static HTML</h1><p>A plain page with JaWS head and tail hooks.</p></main>`)
	_ = rq.TailHTML(w)
	_, _ = io.WriteString(w, "</body></html>")
}

func newBenchmarkPageTemplateHandler(b *testing.B, jw *jaws.Jaws) http.Handler {
	mustAddBenchmarkTemplates(b, jw, `
{{define "benchmarkPageTemplate"}}
<!doctype html>
<html>
<head>
	<title>{{.Dot.Title}}</title>
	{{$.HeadHTML}}
</head>
<body>
	<main>
		<h1>{{.Dot.Title}}</h1>
		<p>{{.Dot.Body}}</p>
	</main>
	{{$.TailHTML}}
</body>
</html>
{{end}}
`)
	return Handler(jw, "benchmarkPageTemplate", &benchmarkPageDot{
		Title: "Template Page",
		Body:  "A full page rendered through html/template and ui.Handler.",
	})
}

func newBenchmarkSimpleWidgetsHandler(b *testing.B, jw *jaws.Jaws) http.Handler {
	mustAddBenchmarkTemplates(b, jw, `
{{define "benchmarkSimpleWidgets"}}
<!doctype html>
<html>
<head>
	<title>{{.Dot.Title}}</title>
	{{$.HeadHTML}}
</head>
<body>
	<main>
		<h1>{{$.Span .Dot.Title}}</h1>
		<section>{{$.Div .Dot.Body}}</section>
		<nav>
			{{$.Button .Dot.Primary "class=\"primary\""}}
			{{$.Button .Dot.Secondary "class=\"secondary\""}}
		</nav>
		<footer>{{$.Span .Dot.Footer}}</footer>
	</main>
	{{$.TailHTML}}
</body>
</html>
{{end}}
`)
	return Handler(jw, "benchmarkSimpleWidgets", &benchmarkSimpleWidgetsDot{
		Title:     "Simple Widgets",
		Body:      "A handful of server-rendered JaWS widgets.",
		Primary:   "Apply",
		Secondary: "Cancel",
		Footer:    "Rendered with Span, Div, and Button helpers.",
	})
}

func newBenchmarkRepeatedWidgetsHandler(b *testing.B, jw *jaws.Jaws) http.Handler {
	mustAddBenchmarkTemplates(b, jw, `
{{define "benchmarkRepeatedWidgets"}}
<!doctype html>
<html>
<head>
	<title>{{.Dot.Title}}</title>
	{{$.HeadHTML}}
</head>
<body>
	<main>
		<h1>{{.Dot.Title}}</h1>
		<ul>
			{{range .Dot.Items}}
			<li>
				{{$.Span .Title "class=\"item-title\""}}
				{{$.Div .Detail "class=\"item-detail\""}}
				{{$.Button .Action "class=\"item-action\""}}
			</li>
			{{end}}
		</ul>
	</main>
	{{$.TailHTML}}
</body>
</html>
{{end}}
`)
	return Handler(jw, "benchmarkRepeatedWidgets", newBenchmarkRepeatedWidgetsDot())
}

func newBenchmarkInputsAndTemplatesHandler(b *testing.B, jw *jaws.Jaws) http.Handler {
	mustAddBenchmarkTemplates(b, jw, `
{{define "benchmarkInputsAndTemplates"}}
<!doctype html>
<html>
<head>
	<title>{{.Dot.Title}}</title>
	{{$.HeadHTML}}
</head>
<body>
	<main>
		<h1>{{$.Span .Dot.TitleBinder}}</h1>
		{{$.Template "benchmarkInputSummary" .Dot "class=\"summary\""}}
		<form>
			<label>Name {{$.Text .Dot.NameBinder}}</label>
			<label>Enabled {{$.Checkbox .Dot.EnabledBinder}}</label>
			<label>Score {{$.Number .Dot.ScoreBinder}}</label>
		</form>
		<section>
			{{range .Dot.Cards}}
				{{$.Template "benchmarkInputCard" . "class=\"card\""}}
			{{end}}
		</section>
	</main>
	{{$.TailHTML}}
</body>
</html>
{{end}}

{{define "benchmarkInputSummary"}}
<aside>
	{{$.Span .Dot.NameBinder}}
	{{$.Button .Dot.TitleBinder "class=\"summary-action\""}}
</aside>
{{end}}

{{define "benchmarkInputCard"}}
<article>
	<header>{{$.Span .Dot.TitleBinder}}</header>
	<label>Note {{$.Text .Dot.NoteBinder}}</label>
	<label>Done {{$.Checkbox .Dot.DoneBinder}}</label>
	<label>Amount {{$.Number .Dot.AmountBinder}}</label>
	{{$.Button .Dot.ActionBinder "class=\"card-action\""}}
</article>
{{end}}
`)
	return Handler(jw, "benchmarkInputsAndTemplates", newBenchmarkInputPageDot())
}

type benchmarkPageDot struct {
	Title string
	Body  string
}

type benchmarkSimpleWidgetsDot struct {
	Title     string
	Body      string
	Primary   string
	Secondary string
	Footer    string
}

type benchmarkRepeatedWidgetsDot struct {
	Title string
	Items []benchmarkRepeatedWidgetItem
}

type benchmarkRepeatedWidgetItem struct {
	Title  string
	Detail string
	Action string
}

func newBenchmarkRepeatedWidgetsDot() *benchmarkRepeatedWidgetsDot {
	dot := &benchmarkRepeatedWidgetsDot{
		Title: "Repeated Widgets",
		Items: make([]benchmarkRepeatedWidgetItem, 24),
	}
	for i := range dot.Items {
		dot.Items[i] = benchmarkRepeatedWidgetItem{
			Title:  fmt.Sprintf("Item %02d", i+1),
			Detail: fmt.Sprintf("Server-rendered detail row %02d", i+1),
			Action: fmt.Sprintf("Open %02d", i+1),
		}
	}
	return dot
}

type benchmarkInputPageDot struct {
	mu sync.RWMutex

	Title   string
	Name    string
	Enabled bool
	Score   float64
	Cards   []*benchmarkInputCardDot

	TitleBinder   bind.Binder[string]
	NameBinder    bind.Binder[string]
	EnabledBinder bind.Binder[bool]
	ScoreBinder   bind.Binder[float64]
}

type benchmarkInputCardDot struct {
	mu sync.RWMutex

	Title  string
	Note   string
	Done   bool
	Amount float64
	Action string

	TitleBinder  bind.Binder[string]
	NoteBinder   bind.Binder[string]
	DoneBinder   bind.Binder[bool]
	AmountBinder bind.Binder[float64]
	ActionBinder bind.Binder[string]
}

func newBenchmarkInputPageDot() *benchmarkInputPageDot {
	dot := &benchmarkInputPageDot{
		Title:   "Inputs And Templates",
		Name:    "Ada",
		Enabled: true,
		Score:   42.5,
		Cards:   make([]*benchmarkInputCardDot, 12),
	}
	dot.TitleBinder = bind.New(&dot.mu, &dot.Title)
	dot.NameBinder = bind.New(&dot.mu, &dot.Name)
	dot.EnabledBinder = bind.New(&dot.mu, &dot.Enabled)
	dot.ScoreBinder = bind.New(&dot.mu, &dot.Score)
	for i := range dot.Cards {
		dot.Cards[i] = newBenchmarkInputCardDot(i)
	}
	return dot
}

func newBenchmarkInputCardDot(idx int) *benchmarkInputCardDot {
	card := &benchmarkInputCardDot{
		Title:  fmt.Sprintf("Card %02d", idx+1),
		Note:   fmt.Sprintf("Editable note %02d", idx+1),
		Done:   idx%2 == 0,
		Amount: float64(idx) + 0.25,
		Action: fmt.Sprintf("Save %02d", idx+1),
	}
	card.TitleBinder = bind.New(&card.mu, &card.Title)
	card.NoteBinder = bind.New(&card.mu, &card.Note)
	card.DoneBinder = bind.New(&card.mu, &card.Done)
	card.AmountBinder = bind.New(&card.mu, &card.Amount)
	card.ActionBinder = bind.New(&card.mu, &card.Action)
	return card
}

func benchmarkPageRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = benchmarkRemoteAddr
	return req
}

func benchmarkCleanupNoscript(b *testing.B, jw *jaws.Jaws, remoteAddr, pageHTML string) {
	b.Helper()
	jawsKey := benchmarkExtractJawsKey(b, pageHTML)
	req := httptest.NewRequest(http.MethodGet, "/jaws/"+jawsKey+"/noscript", nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()

	jw.ServeHTTP(rr, req)
	if got := benchmarkStatusCode(rr); got != http.StatusNoContent {
		b.Fatalf("noscript status = %d, want %d", got, http.StatusNoContent)
	}
}

func benchmarkExtractJawsKey(b *testing.B, pageHTML string) string {
	b.Helper()
	const prefix = `<meta name="jawsKey" content="`
	idx := strings.Index(pageHTML, prefix)
	if idx < 0 {
		b.Fatalf("page did not contain JaWS key meta tag")
	}
	rest := pageHTML[idx+len(prefix):]
	key, _, ok := strings.Cut(rest, `"`)
	if !ok || key == "" {
		b.Fatalf("page contained an empty or unterminated JaWS key meta tag")
	}
	return key
}

func benchmarkStatusCode(rr *httptest.ResponseRecorder) int {
	if rr.Code == 0 {
		return http.StatusOK
	}
	return rr.Code
}

func mustAddBenchmarkTemplates(b *testing.B, jw *jaws.Jaws, text string) {
	b.Helper()
	tmpl, err := template.New("benchmark").Parse(text)
	if err == nil {
		err = jw.AddTemplateLookuper(tmpl)
	}
	if err != nil {
		b.Fatal(err)
	}
}
