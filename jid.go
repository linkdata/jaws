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

func ParseJid(s string) Jid {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return Jid(n)
	}
	return 0
}
