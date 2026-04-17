package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
)

func TestCellButtonUsesCellTagsAndHandlers(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	rq := jaws.NewTestRequest(jw, httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("expected test request")
	}
	defer rq.Close()

	g := newGame(3, 3, 1)
	cell := g.cells[0][0]
	elem := rq.NewElement(ui.NewButton(bind.MakeHTMLGetter(cell)))

	var body bytes.Buffer
	if err := elem.JawsRender(&body, []any{`class="cell"`}); err != nil {
		t.Fatal(err)
	}
	if !elem.HasTag(cell) {
		t.Fatal("expected cell tag to be registered")
	}
	if !elem.HasTag(&g.cells) {
		t.Fatal("expected shared board tag to be registered")
	}

	if err := jaws.CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "0 0 0 flag"); err != nil {
		t.Fatalf("context menu error: %v", err)
	}
	if !cell.flagged {
		t.Fatal("expected cell to be flagged after context menu")
	}
	if g.flags != 1 {
		t.Fatalf("flags = %d, want 1", g.flags)
	}

	other := g.cells[0][1]
	otherElem := rq.NewElement(ui.NewButton(bind.MakeHTMLGetter(other)))
	if err := otherElem.JawsRender(&body, []any{`class="cell"`}); err != nil {
		t.Fatal(err)
	}
	if err := jaws.CallEventHandlers(otherElem.Ui(), otherElem, what.Click, "0 0 0 reveal"); err != nil {
		t.Fatalf("click error: %v", err)
	}
	if !g.started {
		t.Fatal("expected first click to start the game")
	}
	if !other.revealed {
		t.Fatal("expected clicked cell to be revealed")
	}
}
