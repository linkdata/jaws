// Package bind adapts Go values to JaWS getter, setter, HTML, tag, and event
// interfaces.
//
// [New] creates the usual binding from a locker-protected pointer. The pointer
// remains the binding's tag through every [Binder] builder, and each builder
// returns a new chain rather than mutating the earlier value. Widgets bound to
// the same pointer therefore share one dirty identity.
//
// [MakeHTMLGetter] defines the package's HTML conversion boundary. Existing
// [HTMLGetter] values are used unchanged; plain strings and [html/template.HTML]
// are trusted. Its adapters for string-valued [Getter] and [Binder] values and
// [fmt.Stringer] output escape their strings. Escape untrusted text before it
// reaches a trusted form.
package bind
