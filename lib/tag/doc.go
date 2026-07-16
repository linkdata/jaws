// Package tag expands JaWS tag values into comparable keys that identify
// elements during dirtying, broadcasts and event routing.
//
// [TagExpand] rejects expanded key values that cannot be matched reliably as
// tags, including values whose static type is comparable but whose runtime
// contents are not, and otherwise admissible values containing NaN that do not
// equal themselves.
package tag
