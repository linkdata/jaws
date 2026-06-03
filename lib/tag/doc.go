// Package tag expands JaWS tag values into comparable keys that identify
// elements during dirtying, broadcasts and event routing.
//
// The runtime tag-comparability check in [TagExpand] is gated on
// deadlock.Debug, so it (and full statement coverage of this package) is only
// exercised when the tests run with the -race flag or -tags debug. Always test
// with -race.
package tag
