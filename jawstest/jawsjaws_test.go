package jawstest

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
)

func TestJaws_RequestLifecycle(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("nil request")
	}
	if jw.RequestCount() != 1 {
		t.Fatalf("unexpected request count: %d", jw.RequestCount())
	}

	if got := jw.UseRequest(0, hr); got != nil {
		t.Fatal("expected nil for invalid key")
	}
	if got := jw.UseRequest(rq.JawsKey, hr); got != rq {
		t.Fatal("expected claimed request")
	}
	if got := jw.UseRequest(rq.JawsKey, hr); got != nil {
		t.Fatal("expected nil for already-claimed request")
	}
}

func TestJaws_TemplateLookupers(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tmpl := template.Must(template.New("it").Parse(`ok`))
	jw.AddTemplateLookuper(tmpl)
	if got := jw.LookupTemplate("it"); got == nil {
		t.Fatal("expected template")
	}
	jw.RemoveTemplateLookuper(tmpl)
	if got := jw.LookupTemplate("it"); got != nil {
		t.Fatal("expected template removed")
	}
}

func TestNewTestRequestHarness(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	rq := NewTestRequest(jw, nil)
	if rq == nil {
		t.Fatal("nil test request")
	}
	defer rq.Close()

	if rq.Initial() == nil {
		t.Fatal("expected initial request")
	}

	if err := rq.Template("missingtemplate", nil); err == nil {
		t.Fatal("expected missing template error")
	}
}

func TestRequestWriterHelpersFromTemplate(t *testing.T) {
	tj := newTestJaws()
	defer tj.Close()

	tj.AddTemplateLookuper(template.Must(template.New("rwhelper").Parse(`{{$.Span "ok"}}`)))
	rq := tj.newRequest(nil)
	defer rq.Close()

	if err := rq.Template("rwhelper", nil); err != nil {
		t.Fatal(err)
	}
	if got := rq.BodyString(); !strings.Contains(got, `<span id="Jid.`) || !strings.Contains(got, `>ok</span>`) {
		t.Fatalf("unexpected body: %q", got)
	}
}
