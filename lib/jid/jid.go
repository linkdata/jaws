package jid

import (
	"strconv"
	"strings"
)

// Jid is the basis for the HTML id attribute for a UI element within an active
// request.
//
// It is request-scoped, meaning Jid(1) in one request is not the same element
// as Jid(1) in another request.
type Jid int64

// Prefix prefixes HTML IDs based on [Jid] values.
const Prefix = "Jid."

// Invalid is the canonical invalid [Jid] returned by the parsers when text does
// not contain a valid [Jid].
//
// Invalid is exactly Jid(-1); the parsers never return any other negative value.
// [Jid.IsValid] accepts the broader range j >= 0, so any other negative Jid is
// reported invalid by IsValid but is not equal to Invalid.
const Invalid = Jid(-1)

// IsValid reports whether jid can identify an element or the request as a whole.
func (j Jid) IsValid() bool {
	return j >= 0
}

// AppendInt appends the text format of the Jid's numerical value.
//
// The value is appended only for positive Jids; Jid(0) (the whole-request id)
// and negative/invalid Jids append nothing.
func (j Jid) AppendInt(dst []byte) []byte {
	if j > 0 {
		dst = strconv.AppendInt(dst, int64(j), 10)
	}
	return dst
}

// Append appends the unquoted string format of the Jid.
func (j Jid) Append(dst []byte) []byte {
	if j > 0 {
		dst = append(dst, Prefix...)
		dst = j.AppendInt(dst)
	}
	return dst
}

// AppendQuote appends the string format of the Jid surrounded by double quotes.
func (j Jid) AppendQuote(dst []byte) []byte {
	dst = append(dst, '"')
	dst = j.Append(dst)
	dst = append(dst, '"')
	return dst
}

// AppendStartTagAttr appends `<startTag` followed by the quoted [Jid] as an
// HTML id attribute when jid is non-zero.
//
// startTag must be a trusted, syntactically valid HTML tag name: it is written
// verbatim with no escaping or validation and MUST NOT be derived from untrusted
// user data, or it becomes an HTML-injection primitive. The numeric Jid itself
// is always safe.
func (j Jid) AppendStartTagAttr(dst []byte, startTag string) []byte {
	dst = append(dst, '<')
	dst = append(dst, startTag...)
	if j > 0 {
		dst = append(dst, ` id=`...)
		dst = j.AppendQuote(dst)
	}
	return dst
}

// ParseInt parses a Jid integer and returns it as a Jid.
//
// Returns [Invalid] if s is not a valid [Jid] integer or an error occurs.
func ParseInt(s string) Jid {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n >= 0 {
		return Jid(n)
	}
	return Invalid
}

// ParseString parses an unquoted [Jid] string, such as `Jid.2`, and returns
// the corresponding value.
//
// An empty string parses to Jid(0), the whole-request id. Returns [Invalid] if
// s is non-empty and not a valid [Jid] string.
func ParseString(s string) Jid {
	if s == "" {
		return 0
	}
	if strings.HasPrefix(s, Prefix) {
		return ParseInt(s[len(Prefix):])
	}
	return Invalid
}

// String returns the unquoted string representation of the Jid.
func (j Jid) String() string {
	if j > 0 {
		return string(j.Append(nil))
	}
	return ""
}
