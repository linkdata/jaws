// Package bind adapts Go values to JaWS getter, setter, HTML, tag, and event
// interfaces.
//
// [New] creates the usual binding from a locker-protected pointer. The pointer
// remains the binding's tag through every [Binder] builder, and each builder
// returns a new chain rather than mutating the earlier value. Widgets bound to
// the same pointer therefore share one dependency tag.
//
// [MakeHTMLGetter] defines the package's HTML conversion boundary. Existing
// [HTMLGetter] values are used unchanged; plain strings and [html/template.HTML]
// are trusted. Adapters for string-valued [Getter] and [fmt.Stringer] values
// escape their strings. Binders from [New] render through JawsGetHTML: default
// and [Binder.Format] output is escaped, while [Binder.GetHTML] output is trusted.
// A Binder[string] wrapper without JawsGetHTML renders escaped JawsGet output.
// Escape untrusted text before it reaches a trusted form.
//
// The [binding guide] covers field bindings, hooks, adapters, and custom sources.
//
// [binding guide]: https://github.com/linkdata/jaws/blob/main/doc/bindings.md
package bind
