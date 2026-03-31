package tags

type TagGetter interface {
	JawsGetTag(ctx Context) any // Note that the Context may be nil
}
