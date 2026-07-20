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
// Within one request, a widget normally backs at most one live [jaws.Element].
// The HTML-inner widgets, [Img], [Option] and [Template] document support for
// backing multiple live Elements because they retain no Element-specific state;
// their shared getters, handlers and template data must also be safe for those
// calls. Input widgets, [ContainerHelper]-based widgets and [JsVar] require a
// distinct widget value for each live Element. [Register] supports multiple live
// Elements only when its Updater does. Distinct widgets may still share
// synchronized application state.
//
// HTML-inner widgets route content through [bind.MakeHTMLGetter]. Plain strings
// are treated as trusted HTML, while [bind.Getter][string], [bind.Binder][string]
// and [fmt.Stringer] values are escaped. Raw [template.HTMLAttr] params are also
// trusted and written as attributes as-is. Use getter/stringer forms or
// html/template escaping for untrusted user text.
package ui
