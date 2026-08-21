package bind_test

import (
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
)

type makeHTMLGetterStringer struct{ text string }

func (s makeHTMLGetterStringer) String() string { return s.text }

type makeHTMLGetterGetter struct{ text string }

func (g makeHTMLGetterGetter) JawsGet(*jaws.Element) string { return g.text }

type makeHTMLGetterNaNStringer float64

func (makeHTMLGetterNaNStringer) String() string { return "<nan>" }

type makeHTMLGetterNaNGetter float64

func (makeHTMLGetterNaNGetter) JawsGet(*jaws.Element) string { return "<getter>" }

type makeHTMLGetterTaggedSliceStringer []string

func (s makeHTMLGetterTaggedSliceStringer) String() string { return strings.Join(s, "") }

func (makeHTMLGetterTaggedSliceStringer) JawsGetTag() any { return tag.Tag("slice") }

type makeHTMLGetterTaggedNaNGetter float64

func (makeHTMLGetterTaggedNaNGetter) JawsGet(*jaws.Element) string { return "<getter>" }

func (makeHTMLGetterTaggedNaNGetter) JawsGetTag() any { return tag.Tag("getter") }

type makeHTMLGetterLogger struct {
	logged chan error
}

func (*makeHTMLGetterLogger) Info(string, ...any) {}
func (*makeHTMLGetterLogger) Warn(string, ...any) {}

func (l *makeHTMLGetterLogger) Error(_ string, args ...any) {
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if err, isError := args[i+1].(error); ok && key == "err" && isError {
			l.logged <- err
			return
		}
	}
	l.logged <- nil
}

func (l *makeHTMLGetterLogger) wait(t *testing.T) (err error) {
	t.Helper()
	select {
	case err = <-l.logged:
	case <-t.Context().Done():
		t.Fatal("timed out waiting for Logger.Error")
	}
	return
}

func newMakeHTMLGetterRequest(t *testing.T, logger jaws.Logger) *jaws.Request {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	jw.Logger = logger
	rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("NewRequest returned nil")
	}
	return rq
}

func renderMakeHTMLGetterSpan(t *testing.T, rq *jaws.Request, value any) (*jaws.Element, string) {
	t.Helper()
	elem := rq.NewElement(ui.NewSpan(value))
	var rendered strings.Builder
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("JawsRender panicked: %v", recovered)
		}
	}()
	if err := elem.JawsRender(&rendered, nil); err != nil {
		t.Fatalf("JawsRender error: %v", err)
	}
	return elem, rendered.String()
}

func requireMakeHTMLGetterSpan(t *testing.T, elem *jaws.Element, rendered, inner string) {
	t.Helper()
	want := `<span id="` + elem.Jid().String() + `">` + inner + `</span>`
	if rendered != want {
		t.Fatalf("rendered HTML = %q, want %q", rendered, want)
	}
}

func TestMakeHTMLGetterUsesWrappedValueAsTag(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		inner string
	}{
		{name: "Stringer", value: makeHTMLGetterStringer{text: "<stringer>"}, inner: "&lt;stringer&gt;"},
		{name: "Getter", value: makeHTMLGetterGetter{text: "<getter>"}, inner: "&lt;getter&gt;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rq := newMakeHTMLGetterRequest(t, nil)
			elem, rendered := renderMakeHTMLGetterSpan(t, rq, tt.value)
			requireMakeHTMLGetterSpan(t, elem, rendered, tt.inner)
			if tags := rq.TagsOf(elem); len(tags) != 1 || tags[0] != tt.value {
				t.Fatalf("registered tags = %#v, want %#v", tags, []any{tt.value})
			}
		})
	}
}

func TestMakeHTMLGetterNonReflexiveWrappedTagIsReported(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		inner string
	}{
		{name: "Stringer", value: makeHTMLGetterNaNStringer(math.NaN()), inner: "&lt;nan&gt;"},
		{name: "Getter", value: makeHTMLGetterNaNGetter(math.NaN()), inner: "&lt;getter&gt;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := &makeHTMLGetterLogger{logged: make(chan error, 1)}
			rq := newMakeHTMLGetterRequest(t, logger)
			elem, rendered := renderMakeHTMLGetterSpan(t, rq, tt.value)
			requireMakeHTMLGetterSpan(t, elem, rendered, tt.inner)
			if tags := rq.TagsOf(elem); len(tags) != 0 {
				t.Fatalf("registered tags = %#v, want none", tags)
			}
			if logged := logger.wait(t); !errors.Is(logged, tag.ErrNotUsableAsTag) {
				t.Fatalf("logged error = %v, want ErrNotUsableAsTag", logged)
			}
		})
	}
}

func TestMakeHTMLGetterNonReflexiveWrappedTagPanicsWithoutLogger(t *testing.T) {
	rq := newMakeHTMLGetterRequest(t, nil)
	elem := rq.NewElement(ui.NewSpan(makeHTMLGetterNaNStringer(math.NaN())))
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_ = elem.JawsRender(io.Discard, nil)
	}()
	err, ok := recovered.(error)
	if !ok || !errors.Is(err, tag.ErrNotUsableAsTag) {
		t.Fatalf("JawsRender panic = %v, want ErrNotUsableAsTag", recovered)
	}
}

func TestMakeHTMLGetterUsesWrappedTagGetter(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		inner string
		tag   tag.Tag
	}{
		{
			name:  "NonComparableStringer",
			value: makeHTMLGetterTaggedSliceStringer{"<slice>"},
			inner: "&lt;slice&gt;",
			tag:   tag.Tag("slice"),
		},
		{
			name:  "NonReflexiveGetter",
			value: makeHTMLGetterTaggedNaNGetter(math.NaN()),
			inner: "&lt;getter&gt;",
			tag:   tag.Tag("getter"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rq := newMakeHTMLGetterRequest(t, nil)
			elem, rendered := renderMakeHTMLGetterSpan(t, rq, tt.value)
			requireMakeHTMLGetterSpan(t, elem, rendered, tt.inner)
			if tags := rq.TagsOf(elem); len(tags) != 1 || tags[0] != tt.tag {
				t.Fatalf("registered tags = %#v, want %#v", tags, []any{tt.tag})
			}
		})
	}
}
