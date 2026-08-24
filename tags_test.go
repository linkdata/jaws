package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"reflect"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

type parseParamsFloatClickHandler struct {
	value float64
}

func (parseParamsFloatClickHandler) JawsClick(*Element, Click) error {
	return nil
}

func TestParseParamsNonReflexiveHandlerNotTagged(t *testing.T) {
	h := parseParamsFloatClickHandler{value: math.NaN()}
	tags, handlers, attrs := ParseParams([]any{h})
	if len(tags) != 0 {
		t.Fatalf("tags = %#v, want none", tags)
	}
	if len(handlers) != 1 {
		t.Fatalf("handlers = %#v, want one", handlers)
	}
	if got, ok := handlers[0].(parseParamsFloatClickHandler); !ok || !math.IsNaN(got.value) {
		t.Fatalf("handler = %#v, want original NaN-bearing handler", handlers[0])
	}
	if len(attrs) != 0 {
		t.Fatalf("attrs = %#v, want none", attrs)
	}
}

type getterWithMetadata struct {
	metadata any
	next     any
}

func (g getterWithMetadata) JawsGetTag() any { return g.next }

func TestJaws_MustTagExpand(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	if got := jw.MustTagExpand([]any{tag.Tag("a"), tag.Tag("b")}); !reflect.DeepEqual(got, []any{tag.Tag("a"), tag.Tag("b")}) {
		t.Fatalf("MustTagExpand = %#v, want both tags", got)
	}
	awaitTestLoggerQueue(t, jw)
	if logged := logger.snapshot(); len(logged) != 0 {
		t.Fatalf("successful expansion logged %v, want nothing", logged)
	}
}

func TestJaws_MustTagExpandAcyclicRuntimeNonComparableGetter(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	leaf := getterWithMetadata{
		metadata: []string{"leaf"},
		next:     tag.Tag("expected"),
	}
	root := getterWithMetadata{
		metadata: []string{"root"},
		next:     leaf,
	}

	got := jw.MustTagExpand(root)
	want := []any{tag.Tag("expected")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MustTagExpand() = %#v, want %#v", got, want)
	}
}

// TestJaws_MustTagExpandLogsAndReturnsPartial locks in the documented behavior with a
// Logger configured: the error is reported and the tags expanded before the failure are
// still returned, so the caller applies a partial result rather than nothing.
func TestJaws_MustTagExpandLogsAndReturnsPartial(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	// "bad" is a plain string, an illegal tag type, so expansion fails after
	// tag.Tag("ok") has already been collected.
	got := jw.MustTagExpand([]any{tag.Tag("ok"), "bad"})
	loggedErr := logger.next(t)
	if !errors.Is(loggedErr, tag.ErrIllegalTagType) {
		t.Fatalf("logged error = %v, want %v", loggedErr, tag.ErrIllegalTagType)
	}
	if !reflect.DeepEqual(got, []any{tag.Tag("ok")}) {
		t.Fatalf("MustTagExpand = %#v, want the partial result [tag.Tag(\"ok\")]", got)
	}
}

// TestJaws_MustTagExpandPanicsWithoutLogger is the other half of the contract: with no
// Logger, Jaws.MustLog panics, so the partial result never reaches the caller.
func TestJaws_MustTagExpandPanicsWithoutLogger(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	if jw.Logger != nil {
		t.Fatal("expected nil Logger by default")
	}

	defer func() {
		x := recover()
		if e, ok := x.(error); !ok || !errors.Is(e, tag.ErrIllegalTagType) {
			t.Fatalf("recovered %#v, want an error matching %v", x, tag.ErrIllegalTagType)
		}
	}()
	jw.MustTagExpand([]any{tag.Tag("ok"), "bad"})
	t.Fatal("MustTagExpand returned instead of panicking")
}

type fuzzParseParamsWant struct {
	tags     []string
	handlers []string
	attrs    []string
}

type fuzzParseParamsComparableTag struct {
	ID byte
}

type fuzzParseParamsInputHandler struct {
	ID byte
}

func (h fuzzParseParamsInputHandler) JawsInput(*Element, string) error {
	return nil
}

type fuzzParseParamsNonComparableInputHandler struct {
	Data []byte
}

func (h fuzzParseParamsNonComparableInputHandler) JawsInput(*Element, string) error {
	return nil
}

type fuzzParseParamsClickHandler struct {
	ID byte
}

func (h fuzzParseParamsClickHandler) JawsClick(*Element, Click) error {
	return nil
}

type fuzzParseParamsNonComparableClickHandler struct {
	Data []byte
}

func (h fuzzParseParamsNonComparableClickHandler) JawsClick(*Element, Click) error {
	return nil
}

type fuzzParseParamsContextMenuHandler struct {
	ID byte
}

func (h fuzzParseParamsContextMenuHandler) JawsContextMenu(*Element, Click) error {
	return nil
}

type fuzzParseParamsNonComparableContextMenuHandler struct {
	Data []byte
}

func (h fuzzParseParamsNonComparableContextMenuHandler) JawsContextMenu(*Element, Click) error {
	return nil
}

type fuzzParseParamsDualHandler struct {
	ID byte
}

func (h fuzzParseParamsDualHandler) JawsInput(*Element, string) error {
	return nil
}

func (h fuzzParseParamsDualHandler) JawsClick(*Element, Click) error {
	return nil
}

type fuzzParseParamsPointerInputHandler struct {
	ID byte
}

func (h *fuzzParseParamsPointerInputHandler) JawsInput(*Element, string) error {
	return nil
}

type fuzzParseParamsTagGetter struct {
	ID byte
}

func (h fuzzParseParamsTagGetter) JawsGetTag() any {
	return tag.Tag(fmt.Sprintf("taggetter:%d", h.ID))
}

type fuzzParseParamsNonComparableTagGetter []byte

func (h fuzzParseParamsNonComparableTagGetter) JawsGetTag() any {
	return tag.Tag(fmt.Sprintf("non-comparable-tagger:%x", []byte(h)))
}

// fuzzParseParamsInitialAttrOnly covers a param implementing an interface ParseParams
// does not recognize: JawsInitialHTMLAttr is invoked separately for the primary getter,
// so ParseParams must classify this as a tag and nothing else.
type fuzzParseParamsInitialAttrOnly struct {
	ID byte
}

func (h fuzzParseParamsInitialAttrOnly) JawsInitialHTMLAttr(*Element) template.HTMLAttr {
	return ""
}

func FuzzParseParams(f *testing.F) {
	f.Add([]byte{}, "")
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, "a")
	f.Add([]byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, `data-x="1"`)
	f.Add([]byte{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}, "\x00\n\t")

	f.Fuzz(func(t *testing.T, recipe []byte, text string) {
		if len(recipe) > 128 {
			recipe = recipe[:128]
		}
		if len(text) > 64 {
			text = text[:64]
		}

		params, want := fuzzParseParamsBuild(recipe, text)
		tags, handlers, attrs := ParseParams(params)
		gotTags := fuzzParseParamsClassifyTags(t, tags)
		gotHandlers := fuzzParseParamsClassifyHandlers(t, handlers)

		if !reflect.DeepEqual(attrs, want.attrs) {
			t.Fatalf("attrs mismatch\nrecipe=%v text=%q\ngot = %#v\nwant= %#v", recipe, text, attrs, want.attrs)
		}
		if !reflect.DeepEqual(gotHandlers, want.handlers) {
			t.Fatalf("handlers mismatch\nrecipe=%v text=%q\ngot = %#v\nwant= %#v", recipe, text, gotHandlers, want.handlers)
		}
		if !reflect.DeepEqual(gotTags, want.tags) {
			t.Fatalf("tags mismatch\nrecipe=%v text=%q\ngot = %#v\nwant= %#v", recipe, text, gotTags, want.tags)
		}
	})
}

func fuzzParseParamsBuild(recipe []byte, text string) (params []any, want fuzzParseParamsWant) {
	params = make([]any, 0, len(recipe))
	for i, b := range recipe {
		token := fuzzParseParamsToken(i, b, text)
		data := fuzzParseParamsData(i, b, text)
		switch b % 21 {
		case 0:
			params = append(params, nil)
			want.tags = append(want.tags, "nil")
		case 1:
			params = append(params, "string:"+token)
			want.attrs = append(want.attrs, "string:"+token)
		case 2:
			attrs := []string{"strings-a:" + token, "", "strings-b:" + token}
			params = append(params, attrs)
			want.attrs = append(want.attrs, attrs...)
		case 3:
			attr := template.HTMLAttr("htmlattr:" + token)
			params = append(params, attr)
			want.attrs = append(want.attrs, string(attr))
		case 4:
			attrs := []template.HTMLAttr{template.HTMLAttr("htmlattrs-a:" + token), "", template.HTMLAttr("htmlattrs-b:" + token)}
			params = append(params, attrs)
			for _, attr := range attrs {
				want.attrs = append(want.attrs, string(attr))
			}
		case 5:
			fn := InputFn(func(*Element, string) error { return nil })
			params = append(params, fn)
			want.handlers = append(want.handlers, "InputFn")
		case 6:
			var fn InputFn
			params = append(params, fn)
		case 7:
			h := fuzzParseParamsInputHandler{ID: b}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsLabel("input", h.ID))
			want.tags = append(want.tags, fuzzParseParamsLabel("input", h.ID))
		case 8:
			h := fuzzParseParamsNonComparableInputHandler{Data: data}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsDataLabel("nonComparableInput", h.Data))
		case 9:
			h := fuzzParseParamsClickHandler{ID: b}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsLabel("click", h.ID))
			want.tags = append(want.tags, fuzzParseParamsLabel("click", h.ID))
		case 10:
			h := fuzzParseParamsNonComparableClickHandler{Data: data}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsDataLabel("nonComparableClick", h.Data))
		case 11:
			h := fuzzParseParamsContextMenuHandler{ID: b}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsLabel("context", h.ID))
			want.tags = append(want.tags, fuzzParseParamsLabel("context", h.ID))
		case 12:
			h := fuzzParseParamsNonComparableContextMenuHandler{Data: data}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsDataLabel("nonComparableContext", h.Data))
		case 13:
			v := fuzzParseParamsComparableTag{ID: b}
			params = append(params, v)
			want.tags = append(want.tags, fuzzParseParamsLabel("comparableTag", v.ID))
		case 14:
			v := tag.Tag("tag:" + token)
			params = append(params, v)
			want.tags = append(want.tags, "tag:"+string(v))
		case 15:
			v := fuzzParseParamsTagGetter{ID: b}
			params = append(params, v)
			want.tags = append(want.tags, fuzzParseParamsLabel("tagGetter", v.ID))
		case 16:
			v := fuzzParseParamsNonComparableTagGetter(data)
			params = append(params, v)
			want.tags = append(want.tags, fuzzParseParamsDataLabel("nonComparableTagGetter", []byte(v)))
		case 17:
			params = append(params, []int{int(b), i})
		case 18:
			v := fuzzParseParamsInitialAttrOnly{ID: b}
			params = append(params, v)
			want.tags = append(want.tags, fuzzParseParamsLabel("initialAttrOnly", v.ID))
		case 19:
			var h *fuzzParseParamsPointerInputHandler
			params = append(params, h)
			want.handlers = append(want.handlers, "nilPointerInput")
			want.tags = append(want.tags, "nilPointerInput")
		case 20:
			h := fuzzParseParamsDualHandler{ID: b}
			params = append(params, h)
			want.handlers = append(want.handlers, fuzzParseParamsLabel("dual", h.ID))
			want.tags = append(want.tags, fuzzParseParamsLabel("dual", h.ID))
		}
	}
	return params, want
}

func fuzzParseParamsClassifyHandlers(t *testing.T, handlers []any) (labels []string) {
	t.Helper()
	for _, handler := range handlers {
		switch h := handler.(type) {
		case InputFn:
			if h == nil {
				t.Fatal("nil InputFn should not be returned as a handler")
			}
			labels = append(labels, "InputFn")
		case fuzzParseParamsInputHandler:
			labels = append(labels, fuzzParseParamsLabel("input", h.ID))
		case fuzzParseParamsNonComparableInputHandler:
			labels = append(labels, fuzzParseParamsDataLabel("nonComparableInput", h.Data))
		case fuzzParseParamsClickHandler:
			labels = append(labels, fuzzParseParamsLabel("click", h.ID))
		case fuzzParseParamsNonComparableClickHandler:
			labels = append(labels, fuzzParseParamsDataLabel("nonComparableClick", h.Data))
		case fuzzParseParamsContextMenuHandler:
			labels = append(labels, fuzzParseParamsLabel("context", h.ID))
		case fuzzParseParamsNonComparableContextMenuHandler:
			labels = append(labels, fuzzParseParamsDataLabel("nonComparableContext", h.Data))
		case *fuzzParseParamsPointerInputHandler:
			if h != nil {
				labels = append(labels, fuzzParseParamsLabel("pointerInput", h.ID))
			} else {
				labels = append(labels, "nilPointerInput")
			}
		case fuzzParseParamsDualHandler:
			labels = append(labels, fuzzParseParamsLabel("dual", h.ID))
		default:
			t.Fatalf("unexpected handler type %T", handler)
		}
	}
	return labels
}

func fuzzParseParamsClassifyTags(t *testing.T, tags []any) (labels []string) {
	t.Helper()
	for _, tagValue := range tags {
		switch v := tagValue.(type) {
		case nil:
			labels = append(labels, "nil")
		case fuzzParseParamsInputHandler:
			labels = append(labels, fuzzParseParamsLabel("input", v.ID))
		case fuzzParseParamsClickHandler:
			labels = append(labels, fuzzParseParamsLabel("click", v.ID))
		case fuzzParseParamsContextMenuHandler:
			labels = append(labels, fuzzParseParamsLabel("context", v.ID))
		case fuzzParseParamsComparableTag:
			labels = append(labels, fuzzParseParamsLabel("comparableTag", v.ID))
		case tag.Tag:
			labels = append(labels, "tag:"+string(v))
		case fuzzParseParamsTagGetter:
			labels = append(labels, fuzzParseParamsLabel("tagGetter", v.ID))
		case fuzzParseParamsNonComparableTagGetter:
			labels = append(labels, fuzzParseParamsDataLabel("nonComparableTagGetter", []byte(v)))
		case fuzzParseParamsInitialAttrOnly:
			labels = append(labels, fuzzParseParamsLabel("initialAttrOnly", v.ID))
		case *fuzzParseParamsPointerInputHandler:
			if v != nil {
				labels = append(labels, fuzzParseParamsLabel("pointerInput", v.ID))
			} else {
				labels = append(labels, "nilPointerInput")
			}
		case fuzzParseParamsDualHandler:
			labels = append(labels, fuzzParseParamsLabel("dual", v.ID))
		default:
			t.Fatalf("unexpected tag type %T", tagValue)
		}
	}
	return labels
}

func fuzzParseParamsToken(i int, b byte, text string) string {
	return fmt.Sprintf("%d:%02x:%s", i, b, text)
}

func fuzzParseParamsData(i int, b byte, text string) []byte {
	data := []byte{byte(i), b}
	if len(text) > 0 {
		data = append(data, text[0])
	}
	return data
}

func fuzzParseParamsLabel(prefix string, id byte) string {
	return fmt.Sprintf("%s:%d", prefix, id)
}

func fuzzParseParamsDataLabel(prefix string, data []byte) string {
	return fmt.Sprintf("%s:%x", prefix, data)
}

type productionTagRenderUI struct {
	getter any
	elem   *Element
}

func (ui *productionTagRenderUI) JawsRender(elem *Element, _ io.Writer, _ []any) error {
	ui.elem = elem
	// An unusable tag reaches Jaws.Logger through Jaws.MustTagExpand, which is what
	// these tests assert.
	elem.ApplyGetter(ui.getter)
	return nil
}

func (*productionTagRenderUI) JawsUpdate(*Element) {}

type productionNamedFloatTag float64

type productionFunctionTagGetter func() any

func (fn productionFunctionTagGetter) JawsGetTag() any {
	return fn()
}

type productionTagGetterWrapper struct {
	value any
}

func (wrapper productionTagGetterWrapper) JawsGetTag() any {
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
	loggedErr := logger.next(t)
	if !errors.Is(loggedErr, tag.ErrNotUsableAsTag) {
		t.Fatalf("UI rendering error = %v, want %v", loggedErr, tag.ErrNotUsableAsTag)
	}
}

func TestUIRenderResolvesDistinctFunctionTagGetters(t *testing.T) {
	want := tag.Tag("leaf")
	next := []any{want, nil}
	getters := make([]productionFunctionTagGetter, len(next))
	for i := range next {
		i := i
		getters[i] = func() any { return next[i] }
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
	awaitTestLoggerQueue(t, tr.Jaws)
	if logged := logger.snapshot(); len(logged) != 0 {
		t.Fatalf("UI rendering errors = %v", logged)
	}
	if ui.elem == nil || !ui.elem.HasTag(want) {
		t.Fatal("UI rendering did not register the function TagGetter's leaf tag")
	}
}
