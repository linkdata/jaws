// Package ui contains the standard JaWS widget implementations.
//
// The package is intentionally organized around extension-oriented building
// blocks so new widgets can be authored here without reading JaWS core code:
//
//   - [HTMLInner]: base renderer for tags with inner HTML content.
//   - [Input], [InputText], [InputBool], [InputFloat], [InputDate]:
//     typed input helpers that handle event/update flow.
//   - [ContainerHelper]: helper for widgets that render dynamic child UI lists.
//
// Naming follows short widget names (`Span`, `NewSpan`).
//
// Every widget that implements [jaws.UI] is request-scoped.
// Construct a fresh widget for each request, normally by calling a
// [RequestWriter] helper while rendering, and never cache a widget for use by
// multiple requests. Widgets for different requests may refer to the same
// application state, binders, handlers or tags when that shared state is
// synchronized as required.
//
// Most constructors return a pointer ([Option], [Register] and [Template] are the
// value-typed exceptions), and the pointer widgets dereference their fields without
// checking the receiver. JaWS core accepts a typed nil and dispatches to it like any
// other [jaws.UI] value (see [jaws.UI]), so rendering or updating one panics here: no
// widget in this package documents nil-receiver tolerance. The zero value of a widget is
// the supported empty form.
//
// Within one request, a widget normally backs at most one live [jaws.Element].
// The HTML-inner widgets, [Img], [Option] and [Template] document support for backing
// multiple live Elements because they retain no Element-specific state; their
// shared getters, handlers and template data must also be safe for those calls. Input
// widgets, [ContainerHelper]-based widgets and [JsVar] require a distinct widget value
// for each live Element. [Register] supports multiple live Elements only when its Updater
// does. Distinct widgets may still share synchronized application state. This is the
// canonical package-wide classification; each concrete type's documentation states its
// conditions.
//
// A widget needing state keyed to one Element rather than to itself claims the Element's
// state slot with [jaws.SetElementState] while rendering, as [Template] does. That is what
// lets such a widget stay a comparable value the container widgets can reuse as a map key.
//
// HTML-inner widgets route content through [bind.MakeHTMLGetter]. Plain strings
// are treated as trusted HTML, while [bind.Getter][string], [bind.Binder][string]
// and [fmt.Stringer] values are escaped. Raw [template.HTMLAttr] params are also
// trusted and written as attributes as-is. Use getter/stringer forms or
// html/template escaping for untrusted user text.
package ui
