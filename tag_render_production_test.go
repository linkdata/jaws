package jaws

import (
	"errors"
	"io"
	"math"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

type productionTagRenderUI struct {
	getter any
	elem   *Element
}

func (ui *productionTagRenderUI) JawsRender(elem *Element, _ io.Writer, _ []any) (err error) {
	ui.elem = elem
	_, _, err = elem.ApplyGetter(ui.getter)
	return
}

func (*productionTagRenderUI) JawsUpdate(*Element) {}

type productionNamedFloatTag float64

func TestUIRenderRejectsUnreachableTag(t *testing.T) {
	tr := newTestRequest(t)
	logger := &captureErrorLogger{}
	tr.Jaws.Logger = logger
	tagValue := productionNamedFloatTag(math.NaN())
	ui := &productionTagRenderUI{getter: tagValue}

	if err := tr.UI(ui); err != nil {
		t.Fatal(err)
	}
	if ui.elem == nil {
		t.Fatal("UI renderer was not called")
	}
	if !errors.Is(logger.err, tag.ErrNotUsableAsTag) {
		t.Fatalf("UI rendering error = %v, want %v", logger.err, tag.ErrNotUsableAsTag)
	}
}
