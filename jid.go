package jaws

import "strconv"

// Jid is the basis for the HTML `id` attribute for an UI element. It is per-Request, meaning
// Jid(1) in one Request is probably not the same as Jid(1) in another.
type Jid int

func (jid Jid) String() string {
	if jid > 0 {
		return strconv.Itoa(int(jid))
	}
	return ""
}

func (jid Jid) IsZero() bool {
	return jid == 0
}

func (jid Jid) Append(dst []byte) []byte {
	return strconv.AppendInt(dst, int64(jid), 10)
}

func (jid Jid) AppendQuote(dst []byte) []byte {
	dst = append(dst, '"')
	dst = jid.Append(dst)
	return append(dst, '"')
}

func (jid Jid) AppendAttr(dst []byte) []byte {
	if !jid.IsZero() {
		dst = append(dst, ` jid=`...)
		dst = jid.AppendQuote(dst)
	}
	return dst
}

func ParseJid(s string) Jid {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return Jid(n)
	}
	return 0
}
