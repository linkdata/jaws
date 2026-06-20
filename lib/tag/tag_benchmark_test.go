package tag

import (
	"reflect"
	"testing"
)

type benchSelfTagger struct{}

func (t *benchSelfTagger) JawsGetTag(Context) any {
	return t
}

type benchChainTagger struct {
	next any
}

func (t *benchChainTagger) JawsGetTag(Context) any {
	return t.next
}

type benchSliceTagger struct {
	tags []any
}

func (t *benchSliceTagger) JawsGetTag(Context) any {
	return t.tags
}

type benchID struct {
	n int
}

var tagExpandBenchSink []any

func benchmarkTagExpandCase(b *testing.B, tag any) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := TagExpand(nil, tag)
		if err != nil {
			b.Fatal(err)
		}
		tagExpandBenchSink = got
	}
}

func BenchmarkTagExpand(b *testing.B) {
	ids := make([]*benchID, 16)
	flatAny := make([]any, 0, len(ids))
	for i := range ids {
		ids[i] = &benchID{n: i}
		flatAny = append(flatAny, ids[i])
	}

	flatTags8 := []Tag{
		Tag("a"), Tag("b"), Tag("c"), Tag("d"),
		Tag("e"), Tag("f"), Tag("g"), Tag("h"),
	}

	nestedAny := []any{
		ids[0],
		[]any{
			ids[1],
			ids[2],
			[]any{ids[3], ids[4], ids[5]},
		},
		[]any{
			ids[6],
			[]any{ids[7], ids[8], ids[9]},
		},
		ids[10],
	}

	sliceTagger := &benchSliceTagger{
		tags: []any{ids[0], ids[1], Tag("x"), Tag("y")},
	}

	var chainRoot any = Tag("leaf")
	for range 8 {
		chainRoot = &benchChainTagger{next: chainRoot}
	}

	cases := []struct {
		name string
		tag  any
	}{
		{name: "SingleTag", tag: Tag("single")},
		// StructTag exercises the struct-kind runtime comparability check that
		// ensureUsableTag runs (scalar/pointer tags skip it); benchID is a small
		// comparable struct value, not a pointer.
		{name: "StructTag", tag: benchID{n: 1}},
		{name: "FlatTags8", tag: flatTags8},
		{name: "FlatAny16", tag: flatAny},
		{name: "NestedAnyTree", tag: nestedAny},
		{name: "SelfTagger", tag: &benchSelfTagger{}},
		{name: "SliceTagger4", tag: sliceTagger},
		{name: "ChainTaggers8", tag: chainRoot},
	}

	for _, bm := range cases {
		b.Run(bm.name, func(b *testing.B) {
			benchmarkTagExpandCase(b, bm.tag)
		})
	}
}

// benchComparableField is a statically comparable struct whose comparability
// genuinely has to be resolved at runtime through an interface field, which is the
// case ensureUsableTag's probe defends against.
type benchComparableField struct {
	v any
}

var comparableProbeSink bool

// BenchmarkComparableAtRuntime guards the comparableAtRuntime doc claim that the
// recover-based == probe is allocation-free, unlike [reflect.Value.Comparable],
// which allocates while walking the value. Run with -benchmem; the Probe case
// must report 0 allocs/op and the ReflectComparable case is the contrast.
func BenchmarkComparableAtRuntime(b *testing.B) {
	tag := any(benchComparableField{v: 42})

	b.Run("Probe", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			comparableProbeSink = comparableAtRuntime(tag)
		}
	})

	b.Run("ReflectComparable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			comparableProbeSink = reflect.ValueOf(tag).Comparable()
		}
	})
}
