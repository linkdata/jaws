package tag

// TagGetter exposes an object's lazily resolved tags to [TagExpand].
//
// JawsGetTag is the canonical public accessor for an object's tags, and application
// code may call it directly. Callers needing flattened, validated keys should pass the
// object to [TagExpand] rather than interpret the raw return value.
//
// [github.com/linkdata/jaws.Element.ApplyGetter] calls JawsGetTag exactly once per
// invocation, and the standard getter-backed widgets invoke ApplyGetter once during an
// Element's initial render. This is not a global call-count guarantee: [TagExpand],
// dirtying, broadcasts, and application code may call it again.
//
// Except for an explicitly documented initialization phase that returns nil, a
// TagGetter must be idempotent in tag identity. After its first non-nil result, every
// call must return a value that [TagExpand] expands to the same set of keys. Previously
// returned containers must continue expanding to the key set they produced when
// returned and must be treated as read-only. Fresh containers and equivalent
// representations are allowed. Non-idempotent TagGetter implementations are
// unsupported.
//
// JaWS does not serialize JawsGetTag calls. A getter used concurrently must synchronize
// its state and safely publish any returned containers.
//
// [github.com/linkdata/jaws.Request.TagsOf] reports every tag actually registered on an
// Element, including tags added separately from the UI object.
type TagGetter interface {
	// JawsGetTag returns the tag or tags for the implementing object.
	JawsGetTag() any
}
