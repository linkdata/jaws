package jaws

import (
	"strconv"
	"strings"

	"github.com/linkdata/jaws/lib/tag"
)

// Key identifies a JaWS request or session.
//
// A zero Key is invalid and encodes as an empty string. Non-zero keys encode as
// base-32 text for use in JaWS URLs, HTML metadata and session cookies.
type Key uint64

// String returns key in the text form used by JaWS.
func (key Key) String() string {
	return string(appendKey(nil, key))
}

// ParseKey parses a JaWS key from its text form.
//
// Any trailing "/..." path suffix is ignored. It returns zero if s does not
// contain a valid base-32 key.
func ParseKey(s string) Key {
	slashIdx := strings.IndexByte(s, '/')
	if slashIdx < 0 {
		slashIdx = len(s)
	}
	if val, err := strconv.ParseUint(s[:slashIdx], 32, 64); err == nil {
		return Key(val)
	}
	return 0
}

// JawsGetTag prevents a Key from being used as an application tag.
func (key Key) JawsGetTag(tag.Context) any {
	return uint64(key)
}

func appendKey(b []byte, key Key) []byte {
	if key != 0 {
		b = strconv.AppendUint(b, uint64(key), 32)
	}
	return b
}
