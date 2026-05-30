package named

import (
	"cmp"
	"html/template"
	"slices"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
)

func Test_NamedBoolArray(t *testing.T) {
	nba := NewBoolArray(false)
	if len(nba.data) != 0 {
		t.Fatalf("len(data)=%d want 0", len(nba.data))
	}

	nba.Add("1", "one")
	if len(nba.data) != 1 {
		t.Fatalf("len(data)=%d want 1", len(nba.data))
	}
	if nba.data[0].Name() != "1" {
		t.Fatalf("name mismatch: got %q", nba.data[0].Name())
	}
	if nba.data[0].HTML() != template.HTML("one") {
		t.Fatalf("html mismatch: got %q", nba.data[0].HTML())
	}
	if nba.data[0].Checked() {
		t.Fatal("expected unchecked")
	}
	if got := nba.String(); got != `&NamedBoolArray{[&{"1","one",false}]}` {
		t.Fatalf("string mismatch: got %q", got)
	}
	if got := nba.Get(); got != "" {
		t.Fatalf("get mismatch: got %q want empty", got)
	}
	if nba.IsChecked("1") {
		t.Fatal("expected IsChecked(1)=false")
	}
	if nba.IsChecked("2") {
		t.Fatal("expected IsChecked(2)=false")
	}

	nba.Set("1", true)
	if !nba.data[0].Checked() {
		t.Fatal("expected checked true")
	}
	if got := nba.Get(); got != "1" {
		t.Fatalf("get mismatch: got %q want 1", got)
	}
	if !nba.IsChecked("1") {
		t.Fatal("expected IsChecked(1)=true")
	}
	if nba.IsChecked("2") {
		t.Fatal("expected IsChecked(2)=false")
	}

	nba.Add("2", "two")
	nba.Add("2", "also two")
	if len(nba.data) != 3 {
		t.Fatalf("len(data)=%d want 3", len(nba.data))
	}
	if got := nba.String(); got != `&NamedBoolArray{[&{"1","one",true},&{"2","two",false},&{"2","also two",false}]}` {
		t.Fatalf("string mismatch: got %q", got)
	}

	nba.WriteLocked(func(nba []*Bool) []*Bool {
		slices.SortFunc(nba, func(a, b *Bool) int {
			return cmp.Compare(b.Name(), a.Name())
		})
		return nba
	})

	nba.ReadLocked(func(nba []*Bool) {
		if nba[0].Name() != "2" || nba[1].Name() != "2" || nba[2].Name() != "1" {
			t.Fatalf("unexpected order: %q, %q, %q", nba[0].Name(), nba[1].Name(), nba[2].Name())
		}
	})

	if got := nba.Count("1"); got != 1 {
		t.Fatalf("Count(1)=%d want 1", got)
	}
	if got := nba.Count("2"); got != 2 {
		t.Fatalf("Count(2)=%d want 2", got)
	}
	if got := nba.Count("3"); got != 0 {
		t.Fatalf("Count(3)=%d want 0", got)
	}

	if nba.data[0].Checked() || nba.data[1].Checked() || !nba.data[2].Checked() {
		t.Fatal("unexpected checked state after sort/set")
	}

	nbaMulti := NewBoolArray(true)
	nbaMulti.Add("1", "one")
	nbaMulti.Add("2", "two")
	nbaMulti.Add("2", "also two")
	nbaMulti.WriteLocked(func(nba []*Bool) []*Bool {
		slices.SortFunc(nba, func(a, b *Bool) int {
			return cmp.Compare(b.Name(), a.Name())
		})
		return nba
	})
	nbaMulti.Set("1", true)
	nbaMulti.Set("2", true)
	if !nbaMulti.data[0].Checked() || !nbaMulti.data[1].Checked() || !nbaMulti.data[2].Checked() {
		t.Fatal("expected all checked in multi mode")
	}
	if got := nbaMulti.Get(); got != "2" {
		t.Fatalf("multi get=%q want 2", got)
	}

	nba.Set("1", true)
	if nba.data[0].Checked() || nba.data[1].Checked() || !nba.data[2].Checked() {
		t.Fatal("expected only name=1 checked in single mode")
	}
	if got := nba.Get(); got != "1" {
		t.Fatalf("get=%q want 1", got)
	}

	if nba.IsChecked("2") {
		t.Fatal("expected IsChecked(2)=false")
	}
	(nba.data)[1].Set(true)
	if !nba.IsChecked("2") {
		t.Fatal("expected IsChecked(2)=true")
	}

	_, rq := newCoreRequest(t)
	e := rq.NewElement(noopUI{})

	if got := nba.JawsGet(e); got != "2" {
		t.Fatalf("JawsGet=%q want 2", got)
	}
	if err := nba.JawsSet(e, "1"); err != nil {
		t.Fatal(err)
	}
	if got := nba.JawsGet(e); got != "1" {
		t.Fatalf("JawsGet=%q want 1", got)
	}
	if err := nba.JawsSet(e, "1"); err != jaws.ErrValueUnchanged {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
}

func TestBoolArray_SingleSelectDuplicateNames(t *testing.T) {
	// Single-select matches by name: same-named values toggle together and the
	// at-most-one-checked invariant is per distinct name, not per Bool.
	nba := NewBoolArray(false)
	nba.Add("a", "A1")
	nba.Add("a", "A2")
	nba.Add("b", "B")

	if !nba.Set("a", true) {
		t.Fatal("Set(a,true) reported no change")
	}
	if !nba.data[0].Checked() || !nba.data[1].Checked() || nba.data[2].Checked() {
		t.Fatal("both same-named values should be checked together, b unchecked")
	}

	// Selecting a different name deselects every value with a different name,
	// which clears both "a" values at once.
	if !nba.Set("b", true) {
		t.Fatal("Set(b,true) reported no change")
	}
	if nba.data[0].Checked() || nba.data[1].Checked() || !nba.data[2].Checked() {
		t.Fatal("selecting b should deselect both a values and check b")
	}
}

func TestNamedBoolOption_RenderAndUpdateBranches(t *testing.T) {
	_, rq := newCoreRequest(t)

	nba := NewBoolArray(false).Add("1", "one")
	nba.Set("1", true)
	contents := nba.JawsContains(nil)
	if len(contents) != 1 {
		t.Fatalf("want 1 content got %d", len(contents))
	}
	elem := rq.NewElement(contents[0])
	var sb strings.Builder
	if err := elem.JawsRender(&sb, []any{"hidden"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "selected") {
		t.Fatal("expected selected option rendering")
	}
	contents[0].JawsUpdate(elem)
	nba.Set("1", false)
	contents[0].JawsUpdate(elem)
}
