package jaws

import (
	"strconv"
	"strings"
)

// Jid is the basis for the HTML `id` attribute for an UI Element within an active Request.
// It is per-Request, meaning Jid(1) in one Request is not the same as Jid(1) in another.
type Jid uint32

const JidPrefix = "Jid." // String prefixing HTML ID's based on Jid's.
const jidBits = 32

func (jid Jid) Append(dst []byte) []byte {
	dst = append(dst, []byte(JidPrefix)...)
	dst = strconv.AppendUint(dst, uint64(jid), 10)
	return dst
}

func (jid Jid) AppendQuote(dst []byte) []byte {
	dst = append(dst, '"')
	dst = jid.Append(dst)
	return append(dst, '"')
}

func (jid Jid) AppendAttr(dst []byte) []byte {
	if jid != 0 {
		dst = append(dst, ` id=`...)
		dst = jid.AppendQuote(dst)
	}
	return dst
}

func ParseJid(s string) Jid {
	if strings.HasPrefix(s, JidPrefix) {
		if n, err := strconv.ParseUint(s[len(JidPrefix):], 10, jidBits); err == nil && n > 0 {
			return Jid(n)
		}
	}
	return 0
}

func (jid Jid) String() string {
	if jid > 0 {
		buf := make([]byte, 0, len(JidPrefix)+10)
		return string(jid.Append(buf))
	}
	return ""
}
