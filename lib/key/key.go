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

// Parse parses a JaWS key prefix from its text form.
//
// If s contains a slash, tail is the slash and everything after it. The returned
// key is zero if the prefix before tail is not a valid base-32 key.
//
// Decoding is case-insensitive (base-32 'A' and 'a' both decode to 10), while
// [Key.String] and [Append] always emit lowercase. Apart from letter case, the
// prefix must match the canonical text emitted by [Key.String].
func Parse(s string) (key Key, tail string) {
	slashIdx := strings.IndexByte(s, '/')
	keystr := s
	if slashIdx >= 0 {
		keystr = s[:slashIdx]
		tail = s[slashIdx:]
	}
	// A leading '0' is non-canonical: [Key.String] never emits one, and the only
	// value whose text starts with '0' is the all-zero text of the invalid zero Key,
	// so rejecting it also rejects Key(0). keystr is non-empty when err is nil, so
	// indexing it is safe.
	if val, err := strconv.ParseUint(keystr, 32, 64); err == nil && keystr[0] != '0' {
		key = Key(val)
	}
	return
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
