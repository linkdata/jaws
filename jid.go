package jaws

// Jid is the basis for the HTML `id` attribute for an UI element. It is per-Request, meaning
// Jid(1) in one Request is probably not the same as Jid(1) in another.
type Jid int
