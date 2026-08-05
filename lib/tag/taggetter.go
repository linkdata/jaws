package tag

// TagGetter exposes an object's tags to [TagExpand].
//
// [github.com/linkdata/jaws.Element.ApplyGetter], [TagExpand], and APIs that use
// TagExpand for registration, dirtying, or broadcast destinations may call JawsGetTag
// before or after rendering and more than once. Application code may also call it
// directly. The method receives no Request or rendering context. Callers needing
// flattened, validated keys should use [TagExpand] rather than interpret the raw return
// value.
//
// An explicitly documented initialization phase may return a nil interface, which
// expands to no keys. A typed nil is a non-nil interface and follows [TagExpand]'s
// normal rules for its dynamic type. Initialization is not retroactive: it does not
// affect earlier registration, dirtying, or broadcasts. After its first non-nil result,
// every call must return a value that [TagExpand] expands to the same set of keys.
// Previously returned containers must continue expanding to the key set they produced
// when returned and must be treated as read-only. Fresh containers and equivalent
// representations are allowed.
//
// JaWS does not serialize JawsGetTag calls. A getter used concurrently must synchronize
// its state and safely publish any returned containers.
type TagGetter interface {
	// JawsGetTag returns the value that [TagExpand] interprets as the object's tags.
	JawsGetTag() any
}
