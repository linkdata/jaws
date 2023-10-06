package jid

import (
	"strconv"
	"strings"
)

// Jid is the basis for the HTML `id` attribute for an UI Element within an active Request.
// It is per-Request, meaning Jid(1) in one Request is not the same as Jid(1) in another.
type Jid int64

const Prefix = "Jid." // String prefixing HTML ID's based on Jid's.
const Invalid = Jid(-1)

func (jid Jid) IsValid() bool {
	return jid >= 0
}

// AppendInt appends just the text format of the Jid's numerical value.
func (jid Jid) AppendInt(dst []byte) []byte {
	if jid > 0 {
		dst = strconv.AppendInt(dst, int64(jid), 10)
	}
	return dst
}

// Append appends the unquoted string format of the Jid.
func (jid Jid) Append(dst []byte) []byte {
	if jid > 0 {
		dst = append(dst, []byte(Prefix)...)
		dst = jid.AppendInt(dst)
	}
	return dst
}

// AppendQuote appends the string format of the Jid surrounded by double quotes.
func (jid Jid) AppendQuote(dst []byte) []byte {
	dst = append(dst, '"')
	dst = jid.Append(dst)
	dst = append(dst, '"')
	return dst
}

// AppendAttr appends `<startTag` followed by the quoted Jid as a HTML id attribute.
func (jid Jid) AppendStartTagAttr(dst []byte, startTag string) []byte {
	dst = append(dst, '<')
	dst = append(dst, startTag...)
	if jid > 0 {
		dst = append(dst, ` id=`...)
		dst = jid.AppendQuote(dst)
	}
	return dst
}

// ParseInt parses a Jid integer and returns it as a Jid.
//
// Returns jid.Invalid if it's not a valid Jid or an error occurs.
func ParseInt(s string) Jid {
	if n, err := strconv.ParseInt(s, 10, 32); err == nil && n >= 0 {
		return Jid(n)
	}
	return Invalid
}

// ParseString parses an unquoted Jid string (e.g. `Jid.2`) and returns the Jid value (e.g. Jid(2)).
//
// Returns jid.Invalid if it's not a valid Jid string.
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
func (jid Jid) String() string {
	if jid > 0 {
		return string(jid.Append(nil))
	}
	return ""
}
