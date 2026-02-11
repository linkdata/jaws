package ui

import (
	"strings"
	"sync"
	"testing"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
)

func TestRequestWriterRegisteredHelpers(t *testing.T) {
	_, rq := newRequest(t)
	var sb strings.Builder
	rw := rq.Writer(&sb)

	var mu sync.RWMutex
	vbool := true
	vtime, _ := time.Parse("2006-01-02", "2020-01-02")
	vnumber := float64(1.2)
	vstring := "x"
	nba := pkg.NewNamedBoolArray()

	if err := rw.A("a"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Button("b"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Checkbox(pkg.Bind(&mu, &vbool)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Container("x", &testContainer{}); err != nil {
		t.Fatal(err)
	}
	if err := rw.Date(pkg.Bind(&mu, &vtime)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Div("d"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Img("img"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Label("l"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Li("li"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Number(pkg.Bind(&mu, &vnumber)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Password(pkg.Bind(&mu, &vstring)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Radio(pkg.Bind(&mu, &vbool)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Range(pkg.Bind(&mu, &vnumber)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Select(nba); err != nil {
		t.Fatal(err)
	}
	if err := rw.Span("s"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Tbody(&testContainer{}); err != nil {
		t.Fatal(err)
	}
	if err := rw.Td("td"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Text(pkg.Bind(&mu, &vstring)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Textarea(pkg.Bind(&mu, &vstring)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Tr("tr"); err != nil {
		t.Fatal(err)
	}
}
