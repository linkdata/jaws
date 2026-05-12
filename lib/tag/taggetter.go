package tag

// TagGetter exposes dynamic tags during [TagExpand].
type TagGetter interface {
	JawsGetTag(ctx Context) any // Note that the Context may be nil
}
