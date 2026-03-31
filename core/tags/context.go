package tags

// Context can log or panic on tag expansion errors.
// A nil Context causes MustTagExpand to panic on non-nil errors.
type Context interface {
	MustLog(err error)
}
