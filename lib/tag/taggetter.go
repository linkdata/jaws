package tag

// TagGetter exposes dynamic tags during [TagExpand].
//
// Function values may implement TagGetter, but Go does not expose closure
// identity. Recursive function-valued implementations are therefore stopped by
// [TagExpand]'s depth limit and return [ErrTooManyTags].
type TagGetter interface {
	// JawsGetTag returns the dynamic tag or tags for the implementing object.
	//
	// ctx may be nil — [TagExpand] is routinely called with a nil [Context] — so
	// implementations must not dereference it unconditionally.
	JawsGetTag(ctx Context) any
}
