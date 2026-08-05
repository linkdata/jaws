package tag

import (
	"testing"
)

// freshSliceTagger returns an equal but freshly allocated container on every call,
// which the TagGetter contract explicitly permits: only the expanded key set has to
// stay the same.
type freshSliceTagger struct {
	keys []Tag
}

func (g *freshSliceTagger) JawsGetTag() any {
	out := make([]any, 0, len(g.keys))
	for _, k := range g.keys {
		out = append(out, k)
	}
	return out
}

func TestTagGetter_FreshContainersExpandToTheSameKeySet(t *testing.T) {
	g := &freshSliceTagger{keys: []Tag{Tag("a"), Tag("b")}}
	first, err := TagExpand(g)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, first, []any{Tag("a"), Tag("b")})

	second, err := TagExpand(g)
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, second, first)
}

// sharedLeafTagger lets one getter be reached from two points of a single graph.
type sharedLeafTagger struct {
	key Tag
}

func (g *sharedLeafTagger) JawsGetTag() any { return g.key }

// TestTagGetter_RepeatedGraphOccurrenceDeduplicates asserts the observable outcome of a
// getter reached more than once in one expansion: a single deduplicated key set. It
// deliberately does not assert how many times JawsGetTag was called, since the contract
// makes no such guarantee and a future per-expansion cache must stay compliant.
func TestTagGetter_RepeatedGraphOccurrenceDeduplicates(t *testing.T) {
	leaf := &sharedLeafTagger{key: Tag("leaf")}
	got, err := TagExpand([]any{leaf, Tag("other"), []any{leaf}})
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, got, []any{Tag("leaf"), Tag("other")})
}

func TestTagGetter_StableNestedAndCyclicGettersRepeatIdentically(t *testing.T) {
	t.Run("nested", func(t *testing.T) {
		inner := &sharedLeafTagger{key: Tag("inner")}
		outer := testDeepTagGetter{next: []any{Tag("outer"), inner}}
		first, err := TagExpand(outer)
		if err != nil {
			t.Fatal(err)
		}
		assertTagSetEqual(t, first, []any{Tag("outer"), Tag("inner")})
		second, err := TagExpand(outer)
		if err != nil {
			t.Fatal(err)
		}
		assertTagSetEqual(t, second, first)
	})

	t.Run("mutual cycle", func(t *testing.T) {
		a := &testMutualSliceTagger{name: "a"}
		b := &testMutualSliceTagger{name: "b", next: a}
		a.next = b
		// A closed cycle yields the participating getters as keys, so repeating the
		// expansion must produce that same pair.
		first, err := TagExpand(a)
		if err != nil {
			t.Fatal(err)
		}
		assertTagSetEqual(t, first, []any{a, b})
		second, err := TagExpand(a)
		if err != nil {
			t.Fatal(err)
		}
		assertTagSetEqual(t, second, first)
	})
}
