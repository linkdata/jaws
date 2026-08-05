// Package tag expands JaWS tag values into comparable keys that identify
// elements during dirtying, broadcasts and event routing.
//
// [TagExpand] rejects expanded key values that cannot be matched reliably as
// tags, including values whose static type is comparable but whose runtime
// contents are not, and otherwise admissible values containing NaN that do not
// equal themselves.
//
// [TagGetter] defines the contract an object implements to report its own tags,
// including the idempotent tag identity every implementation owes while its tags are
// registered on a live element. Expansion takes no request context: a tag value
// expands the same way regardless of which request or goroutine expands it.
package tag
