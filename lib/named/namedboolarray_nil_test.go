package named_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/ui"
)

func TestBoolArrayWriteLockedDiscardsNilElements(t *testing.T) {
	bools := named.NewBoolArray(false).Add("a", "A").Add("b", "B")
	bools.Set("b", true)
	bools.WriteLocked(func(values []*named.Bool) []*named.Bool {
		return []*named.Bool{nil, values[1], nil, values[0], nil}
	})

	var values []*named.Bool
	bools.ReadLocked(func(current []*named.Bool) {
		values = append(values, current...)
	})
	if len(values) != 2 {
		t.Fatalf("len(values) = %d, want 2", len(values))
	}
	if got := values[0].Name(); got != "b" {
		t.Fatalf("values[0].Name() = %q, want b", got)
	}
	if got := values[1].Name(); got != "a" {
		t.Fatalf("values[1].Name() = %q, want a", got)
	}
	if got := bools.Count("a"); got != 1 {
		t.Fatalf("Count(a) = %d, want 1", got)
	}
	if got := bools.Get(); got != "b" {
		t.Fatalf("Get() = %q, want b", got)
	}
	if !bools.IsChecked("b") {
		t.Fatal("IsChecked(b) = false, want true")
	}
	const wantString = `&BoolArray{[&Bool{"b","B",true},&Bool{"a","A",false}]}`
	if got := bools.String(); got != wantString {
		t.Fatalf("String() = %q, want %q", got, wantString)
	}
	if !bools.Set("a", true) {
		t.Fatal("Set(a, true) reported no change")
	}
	if got := bools.Get(); got != "a" {
		t.Fatalf("Get() after Set(a, true) = %q, want a", got)
	}

	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(ui.NewSelect(bools))
	var rendered strings.Builder
	if err = elem.JawsRender(&rendered, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">A</option>", ">B</option>"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("rendered select = %q, want %q", rendered.String(), want)
		}
	}
}
