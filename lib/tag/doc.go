// Package tag expands JaWS tag values into comparable keys that identify
// elements during dirtying, broadcasts and event routing.
//
// [TagExpand] rejects expanded key values that are not usable as hashable tags,
// including values whose static type is comparable but whose runtime contents
// are not and values such as NaN that do not equal themselves.
package tag
