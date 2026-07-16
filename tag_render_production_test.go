package jaws

import (
	"errors"
	"io"
	"math"
	"reflect"
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

type productionFunctionTagGetter func(tag.Context) any

func (fn productionFunctionTagGetter) JawsGetTag(ctx tag.Context) any {
	return fn(ctx)
}

type productionTagGetterWrapper struct {
	value any
}

func (wrapper productionTagGetterWrapper) JawsGetTag(tag.Context) any {
	return wrapper.value
}

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

func TestUIRenderResolvesDistinctFunctionTagGetters(t *testing.T) {
	want := tag.Tag("leaf")
	next := []any{want, nil}
	getters := make([]productionFunctionTagGetter, len(next))
	for i := range next {
		i := i
		getters[i] = func(tag.Context) any { return next[i] }
	}
	leafGetter := getters[0]
	rootGetter := getters[1]
	next[1] = leafGetter
	if rootPtr, leafPtr := reflect.ValueOf(rootGetter).Pointer(), reflect.ValueOf(leafGetter).Pointer(); rootPtr != leafPtr {
		t.Skipf("compiler emitted distinct code pointers %#x and %#x", rootPtr, leafPtr)
	}

	tr := newTestRequest(t)
	logger := &captureErrorLogger{}
	tr.Jaws.Logger = logger
	ui := &productionTagRenderUI{
		getter: productionTagGetterWrapper{value: rootGetter},
	}
	if err := tr.UI(ui); err != nil {
		t.Fatal(err)
	}
	if logger.err != nil {
		t.Fatalf("UI rendering error = %v", logger.err)
	}
	if ui.elem == nil || !ui.elem.HasTag(want) {
		t.Fatal("UI rendering did not register the function TagGetter's leaf tag")
	}
}
