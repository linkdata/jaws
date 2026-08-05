package tag

// TagGetter exposes an object's tags to [TagExpand].
//
// JawsGetTag is the canonical public accessor for an object's tags, and application
// code may call it directly. Callers needing flattened, validated keys should pass the
// object to [TagExpand] rather than interpret the raw return value.
//
// [github.com/linkdata/jaws.Element.ApplyGetter] invokes JawsGetTag to obtain a tag
// candidate, then expands that candidate for registration. This expansion may invoke
// JawsGetTag again when the candidate is itself a TagGetter or contains one. Standard
// getter-backed widgets invoke ApplyGetter once during an Element's initial render,
// but [TagExpand], dirtying, broadcasts, and application code may invoke JawsGetTag
// before or after that. Expansion supplies no Request or rendering context, and there
// is no call-count guarantee.
//
// An explicitly documented initialization phase may return a nil interface, which
// expands to no keys. A typed nil is a non-nil interface and follows [TagExpand]'s
// normal rules for its dynamic type. Later initialization does not replay earlier
// expanding registration, dirtying, or broadcast operations. After its first non-nil
// result, every call must return a value that [TagExpand] expands to the same set of
// keys. Previously returned containers must continue expanding to the key set they
// produced when returned and must be treated as read-only. Fresh containers and
// equivalent representations are allowed. Non-idempotent TagGetter implementations
// are unsupported.
//
// JaWS does not serialize JawsGetTag calls. A getter used concurrently must synchronize
// its state and safely publish any returned containers.
//
// [github.com/linkdata/jaws.Request.TagsOf] reports every tag actually registered on an
// Element, including tags added separately from the UI object.
type TagGetter interface {
	// JawsGetTag returns the value that [TagExpand] interprets as the object's tags.
	JawsGetTag() any
}
