// Package key implements JaWS key encoding.
package key

import (
	"strconv"
	"strings"
)

// Key identifies a JaWS request or session.
//
// A zero Key is invalid and encodes as an empty string. Non-zero keys encode as
// base-32 text for use in JaWS URLs, HTML metadata and session cookies.
type Key uint64

// String returns key in the text form used by JaWS.
func (key Key) String() string {
	return string(Append(nil, key))
}

// Parse parses a JaWS key from its text form.
//
// Any trailing "/..." path suffix is ignored. It returns zero if s does not
// contain a valid base-32 key.
func Parse(s string) Key {
	slashIdx := strings.IndexByte(s, '/')
	if slashIdx < 0 {
		slashIdx = len(s)
	}
	if val, err := strconv.ParseUint(s[:slashIdx], 32, 64); err == nil {
		return Key(val)
	}
	return 0
}

// Append appends key in the text form used by JaWS to b.
//
// A zero Key (the invalid key) appends nothing and returns b unchanged, matching
// [Key.String], which encodes a zero Key as the empty string.
func Append(b []byte, key Key) []byte {
	if key != 0 {
		b = strconv.AppendUint(b, uint64(key), 32)
	}
	return b
}
