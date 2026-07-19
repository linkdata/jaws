package jawstree

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
)

var (
	_ jaws.UI           = (*Tree)(nil)
	_ jaws.InputHandler = (*Tree)(nil)
)

// JawsRender writes the hidden tree container and schedules its browser
// initialization.
//
// The current selection is included in the initializer, so a freshly rendered client
// is correct before any update arrives. A Tree is shared UI state: one *Tree may be
// rendered by many requests, each getting its own element.
func (t *Tree) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	// Register the dirty tag and read the initial payload before the element freezes.
	// Reading the selection under RLock keeps the payload consistent with the tag even
	// if another request sharing this Tree mutates it concurrently.
	elem.Tag(t)
	attrs := elem.ApplyParams(params)
	t.RLock()
	initData := t.initPayloadLocked(elem.Jid().String())
	t.RUnlock()

	var b []byte
	b = append(b, "\n<div id="...)
	b = elem.Jid().AppendQuote(b)
	b = htmlio.AppendAttrs(b, attrs)
	b = append(b, " hidden></div>"...)
	if _, err = w.Write(b); err == nil {
		elem.JsCall("jawstreeInit", initData)
	}
	return
}

// JawsUpdate pushes the current selection to this element's client.
//
// It is safe to call concurrently with the browser event handling that mutates
// selection, and never rebuilds the tree, so the client's local expansion state is
// preserved.
func (t *Tree) JawsUpdate(elem *jaws.Element) {
	t.RLock()
	payload := t.selectionPayloadLocked(elem.Jid().String())
	t.RUnlock()
	elem.JsCall("jawstreeSelection", payload)
}

// JawsInput applies a browser selection change to the shared Tree.
//
// On a real change it dirties the Tree so every rendered client reconverges. On a
// rejected, malformed, or no-op change it resynchronizes only the originating client,
// whose optimistic view may be ahead of the server, leaving peers untouched. It
// always returns nil: a routine client rejection is logged, not surfaced as an alert.
func (t *Tree) JawsInput(elem *jaws.Element, value string) error {
	var msg struct {
		D *struct {
			Add    []int `json:"add"`
			Remove []int `json:"remove"`
		} `json:"d"`
		S *[]int `json:"s"`
		B string `json:"b"`
	}
	var err error
	changed := false
	if err = json.Unmarshal([]byte(value), &msg); err == nil {
		t.Lock()
		switch {
		case msg.D != nil:
			changed, err = t.applyClientDelta(msg.D.Add, msg.D.Remove)
		case msg.S != nil:
			changed, err = t.applyClientAbsolute(*msg.S)
		case msg.B != "":
			var indices []int
			if indices, err = decodeSelectionBitmap(msg.B, len(t.byIndex)); err == nil {
				changed, err = t.applyClientAbsolute(indices)
			}
		default:
			err = fmt.Errorf("%w: empty selection payload", ErrPathRejected)
		}
		t.Unlock()
	} else {
		err = fmt.Errorf("%w: malformed payload: %v", ErrPathRejected, err)
	}

	if err == nil && changed {
		// Fan the new authoritative selection out to every rendered client, including
		// this one, where the reconcile is an idempotent no-op.
		elem.Dirty(t)
		return nil
	}
	// A reject, malformed frame, or no-op: the origin's optimistic DOM may be ahead of
	// the server, so snap only this origin back to the authoritative selection.
	if err != nil {
		_ = elem.Jaws.Log(err)
	}
	t.RLock()
	payload := t.selectionPayloadLocked(elem.Jid().String())
	t.RUnlock()
	elem.JsCall("jawstreeSelection", payload)
	// Return nil so a routine client reject is not turned into a browser alert by the
	// event loop; it has been logged and the origin resynchronized.
	return nil
}
