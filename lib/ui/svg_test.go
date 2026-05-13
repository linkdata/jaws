package ui

import (
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
)

func TestSVGRender(t *testing.T) {
	_, rq := newCoreRequest(t)
	svg := NewSVG(testHTMLGetter(`<circle cx="1" cy="2" r="3"></circle>`))

	_, got := renderUI(t, rq, svg, `viewBox="0 0 10 10"`)
	mustMatch(t, `^<svg id="Jid\.[0-9]+" viewBox="0 0 10 10"><circle cx="1" cy="2" r="3"></circle></svg>$`, got)
}

func TestRequestWriterSVG(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	if err := rw.SVG(`<path d="M0 0L1 1"></path>`, `viewBox="0 0 1 1"`); err != nil {
		t.Fatal(err)
	}
	mustMatch(t, `^<svg id="Jid\.[0-9]+" viewBox="0 0 1 1"><path d="M0 0L1 1"></path></svg>$`, sb.String())
}

func TestSVGContainerRenderAndUpdate(t *testing.T) {
	_, rq := newCoreRequest(t)
	circle1 := testSVGChild{tag: "circle", attr: `r="1"`}
	circle2 := testSVGChild{tag: "circle", attr: `r="2"`}
	rect := testSVGChild{tag: "rect", attr: `width="3" height="4"`}

	tc := &testContainer{contents: []jaws.UI{circle1}}
	svg := NewSVGContainer(tc)
	elem, got := renderUI(t, rq, svg, `viewBox="0 0 10 10"`)
	mustMatch(t, `^<svg id="Jid\.[0-9]+" viewBox="0 0 10 10"><circle id="Jid\.[0-9]+" r="1"></circle></svg>$`, got)

	tc.contents = []jaws.UI{circle1, circle2, rect}
	svg.JawsUpdate(elem)
	if len(svg.contents) != 3 {
		t.Fatalf("contents = %d, want 3", len(svg.contents))
	}

	removedJid := svg.contents[0].Jid()
	tc.contents = []jaws.UI{rect, circle2}
	svg.JawsUpdate(elem)
	if got := rq.GetElementByJid(removedJid); got != nil {
		t.Fatalf("expected removed SVG child %v to be deleted from request", removedJid)
	}
}

func TestRequestWriterSVGContainer(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}
	tc := &testContainer{contents: []jaws.UI{testSVGChild{tag: "path", attr: `d="M0 0"`}}}

	if err := rw.SVGContainer(tc, `viewBox="0 0 1 1"`); err != nil {
		t.Fatal(err)
	}
	mustMatch(t, `^<svg id="Jid\.[0-9]+" viewBox="0 0 1 1"><path id="Jid\.[0-9]+" d="M0 0"></path></svg>$`, sb.String())
}

type testSVGChild struct {
	tag  string
	attr string
}

func (u testSVGChild) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, u.tag)
	if u.attr != "" {
		b = append(b, ' ')
		b = append(b, u.attr...)
	}
	b = append(b, "></"...)
	b = append(b, u.tag...)
	b = append(b, '>')
	_, err = w.Write(b)
	return
}

func (testSVGChild) JawsUpdate(elem *jaws.Element) {}
