package jaws

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

type pathUpdateUI struct{}

func (pathUpdateUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (pathUpdateUI) JawsUpdate(*Element)                         {}
func (pathUpdateUI) JawsUpdatePaths(*Element, []string)          {}

func TestDirtyPathSet(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "duplicate", paths: []string{"a", "a"}, want: []string{"a"}},
		{name: "ancestor first", paths: []string{"a", "a.b"}, want: []string{"a"}},
		{name: "ancestor later", paths: []string{"a.b", "a.c", "a"}, want: []string{"a"}},
		{name: "component boundary", paths: []string{"a", "ab.c"}, want: []string{"a", "ab.c"}},
		{name: "root", paths: []string{"a", "", "b"}, want: []string{""}},
		{name: "byte limit", paths: []string{strings.Repeat("a", maxDirtyPathBytes+1)}, want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var set dirtyPathSet
			for _, path := range tt.paths {
				set.add(path)
			}
			if !reflect.DeepEqual(set.paths, tt.want) {
				t.Fatalf("paths = %q, want %q", set.paths, tt.want)
			}
		})
	}
	var set dirtyPathSet
	for i := range maxDirtyPaths + 1 {
		set.add(strings.Repeat("x", i+1))
	}
	if !reflect.DeepEqual(set.paths, []string{""}) {
		t.Fatalf("count overflow paths = %q, want root", set.paths)
	}
}

func TestDirtyPathPendingRequestsAndSource(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.updateTicker.Stop()
	serveDone := make(chan struct{})
	go func() {
		jw.Serve()
		close(serveDone)
	}()
	defer func() {
		jw.Close()
		<-serveDone
	}()
	waitForServeLoop(t, jw)

	selector := tag.Tag("shared")
	newPending := func() (*Request, *Element) {
		t.Helper()
		rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		elem := rq.NewElement(pathUpdateUI{})
		elem.Tag(selector)
		return rq, elem
	}
	first, source := newPending()
	second, _ := newPending()

	jw.DirtyPath(selector, "a.x")
	if got := jw.distributeDirt(); got != 1 {
		t.Fatalf("first distributeDirt() = %d, want 1", got)
	}
	jw.DirtyPath(selector, "a.y")
	jw.DirtyPath(source, "a")
	if got := jw.distributeDirt(); got != 2 {
		t.Fatalf("second distributeDirt() = %d, want 2", got)
	}
	if got := first.makePathUpdateList(); len(got) != 1 || got[0].elem != source || !reflect.DeepEqual(got[0].paths, []string{"a"}) {
		t.Fatalf("source updates = %#v, want root of a on source", got)
	}
	if got := second.makePathUpdateList(); len(got) != 1 || !reflect.DeepEqual(got[0].paths, []string{"a.x", "a.y"}) {
		t.Fatalf("peer updates = %#v, want both paths", got)
	}
	if got := second.makePathUpdateList(); len(got) != 0 {
		t.Fatalf("second drain = %#v, want empty", got)
	}
}

func TestDirtyPathPendingRequestsCoalesceRootWithoutUnrelatedWork(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	selector := tag.Tag("shared")
	subscribed := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	elem := subscribed.NewElement(pathUpdateUI{})
	elem.Tag(selector)
	unrelated := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	unrelated.NewElement(pathUpdateUI{}).Tag(tag.Tag("other"))

	for _, path := range []string{"a", "", "b"} {
		jw.DirtyPath(selector, path)
		if got := jw.distributeDirt(); got != 1 {
			t.Fatalf("distributeDirt() after %q = %d, want 1", path, got)
		}
		if len(unrelated.todoDirt) != 0 || len(unrelated.todoPaths) != 0 {
			t.Fatalf("unrelated Request retained dirt after %q: tags=%d paths=%d", path, len(unrelated.todoDirt), len(unrelated.todoPaths))
		}
	}
	if got := subscribed.makePathUpdateList(); len(got) != 1 || got[0].elem != elem || !reflect.DeepEqual(got[0].paths, []string{""}) {
		t.Fatalf("subscribed updates = %#v, want root path on subscribed Element", got)
	}
}

func TestDirtyPendingRequestKeepsLateTagOnce(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	selector := tag.Tag("late")
	rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	for range 3 {
		jw.Dirty(selector)
		if got := jw.distributeDirt(); got != 1 {
			t.Fatalf("distributeDirt() = %d, want 1", got)
		}
	}
	if got := len(rq.todoDirt); got != 1 {
		t.Fatalf("pending selectors = %d, want 1", got)
	}
	if rq.todoDirtSeen == nil {
		t.Fatal("pending selectors have no membership set")
	}
	elem := rq.NewElement(pathUpdateUI{})
	elem.Tag(selector)
	if got := rq.makeUpdateList(); len(got) != 1 || got[0] != elem {
		t.Fatalf("late tag updates = %#v, want Element", got)
	}
	if rq.todoDirtSeen != nil {
		t.Fatal("drained selectors retain their membership set")
	}
}

func TestDirtyPathCloseDropsSelectors(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	selector := new(int)
	jw.DirtyPath(selector, "value")
	jw.Close()
	jw.DirtyPath(selector, "other")
	if len(jw.dirtyPaths) != 0 {
		t.Fatalf("closed Jaws retained %d path selectors", len(jw.dirtyPaths))
	}
}
