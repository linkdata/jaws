// Package ui contains the standard JaWS widget implementations.
//
// The package is intentionally organized around extension-oriented building
// blocks so new widgets can be authored here without reading JaWS core code:
//
//   - [HTMLInner]: base renderer for tags with inner HTML content.
//   - [Input], [InputText], [InputBool], [InputFloat], [InputDate]:
//     typed input helpers that handle event/update flow.
//   - [Container], [Tbody], [Select]: value widgets for dynamic child UI lists.
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
// Most constructors return a pointer. [Container], [Tbody], [Select], [Option],
// [Register] and [Template] are value-typed exceptions. Always use Container, Tbody,
// Select and Template as values, as their constructors return them; taking their
// addresses is unsupported because it changes container reuse from definition equality
// to pointer identity.
//
// The Container, Tbody and Select definitions are immutable. Tbody embeds a Container
// configured by its constructor; Select keeps its typed handler private and uses an
// equivalent Container definition internally. The child provider or handler is an
// interface field, so its dynamic value must be comparable at runtime and equal to
// itself. Application data containing a slice, map or function belongs behind a stable
// pointer; rebuilding with that same pointer gives the widget the same identity while
// the synchronized application state behind it may change. Allocate a fresh widget value
// for each request even when several requests refer to the same application object.
//
// JaWS core accepts a typed nil and dispatches to it like any other [jaws.UI] value (see
// [jaws.UI]). No pointer widget in this package documents nil-receiver tolerance. Consult
// each concrete type for its zero-value behavior. The container-family values
// intentionally require a provider: rendering or updating a zero Container, Tbody or
// Select panics when it calls the nil provider. [Select.JawsInput] alone treats its nil
// handler as a no-op. A typed-nil provider is dispatched normally and must tolerate its
// nil receiver itself.
//
// Within one request, a widget normally backs at most one live [jaws.Element].
// The HTML-inner widgets, [Img], [Option] and [Template] document support for backing
// multiple live Elements because they retain no Element-specific state; their shared
// getters, handlers and template data must also be safe for those calls. Container,
// Tbody and Select also support multiple live Elements: each Element owns an independent
// container reconciliation state. Input widgets and [JsVar] require a distinct widget
// value for each live Element. [Register] supports multiple live Elements only when its
// Updater does. Distinct widgets may still share synchronized application state. This is
// the canonical package-wide classification; each concrete type's documentation states
// its conditions.
//
// A widget needing state keyed to one Element rather than to itself claims the Element's
// state slot with [jaws.SetElementState] while rendering. Template keeps its owned
// generation there; Container, Tbody and Select keep their child reconciliation state
// there. This lets a containing widget reuse rebuilt equal values while each live Element
// remains independent. The container family also claims missing state lazily when used
// through update-only [Register]; that state has no render-time dirty tag.
//
// HTML-inner widgets route content through [bind.MakeHTMLGetter]. Plain strings
// are treated as trusted HTML, while [bind.Getter][string], [bind.Binder][string]
// and [fmt.Stringer] values are escaped. Raw [template.HTMLAttr] params are also
// trusted and written as attributes as-is. Use getter/stringer forms or
// html/template escaping for untrusted user text.
package ui
