package core

import (
	"bufio"
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCoverage_IDAndLookupHelpers(t *testing.T) {
	NextJid = 0
	if a, b := NextID(), NextID(); b <= a {
		t.Fatalf("expected increasing ids, got %d then %d", a, b)
	}
	if got := string(AppendID([]byte("x"))); !strings.HasPrefix(got, "x") || len(got) <= 1 {
		t.Fatalf("unexpected append id result %q", got)
	}
	if got := MakeID(); !strings.HasPrefix(got, "jaws.") {
		t.Fatalf("unexpected id %q", got)
	}

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tmpl := template.Must(template.New("it").Parse(`ok`))
	jw.AddTemplateLookuper(tmpl)
	if got := jw.LookupTemplate("it"); got == nil {
		t.Fatal("expected found template")
	}
	if got := jw.LookupTemplate("missing"); got != nil {
		t.Fatal("expected missing template")
	}
	jw.RemoveTemplateLookuper(nil)
	jw.RemoveTemplateLookuper(tmpl)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}
	if got := jw.RequestCount(); got != 1 {
		t.Fatalf("expected one request, got %d", got)
	}
	jw.recycle(rq)
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("expected zero requests, got %d", got)
	}
}

func TestCoverage_CookieParseAndIP(t *testing.T) {
	h := http.Header{}
	h.Add("Cookie", `a=1; jaws=`+JawsKeyString(11)+`; x=2`)
	h.Add("Cookie", `jaws="`+JawsKeyString(12)+`"`)
	h.Add("Cookie", `jaws=not-a-key`)

	ids := getCookieSessionsIds(h, "jaws")
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 12 {
		t.Fatalf("unexpected cookie ids %#v", ids)
	}

	if got := parseIP("127.0.0.1:1234"); !got.IsValid() {
		t.Fatalf("expected parsed host:port ip, got %v", got)
	}
	if got := parseIP("::1"); !got.IsValid() {
		t.Fatalf("expected parsed direct ip, got %v", got)
	}
	if got := parseIP(""); got.IsValid() {
		t.Fatalf("expected invalid ip for empty remote addr, got %v", got)
	}
}

func TestCoverage_NonZeroRandomAndPanic(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// First random value is zero, second is one.
	zeroThenOne := append(make([]byte, 8), []byte{1, 0, 0, 0, 0, 0, 0, 0}...)
	jw.kg = bufio.NewReader(bytes.NewReader(zeroThenOne))
	if got := jw.nonZeroRandomLocked(); got != 1 {
		t.Fatalf("unexpected non-zero random value %d", got)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on random source read error")
		}
	}()
	jw.kg = bufio.NewReader(errReader{})
	_ = jw.nonZeroRandomLocked()
}
