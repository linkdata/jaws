// Package ui contains the standard JaWS widget implementations.
//
// Its main building blocks are:
//
//   - [HTMLInner]: base renderer for tags with inner HTML content.
//   - [Input], [InputText], [InputBool], [InputDate]:
//     typed input helpers that handle event/update flow.
//   - [Number], [Range]: type-preserving numeric input widgets.
//   - [Container], [Tbody], [Select]: value widgets for dynamic child UI lists.
//
// Every widget implementing [jaws.UI] is request-scoped. Construct fresh widgets
// for each request. Widgets from different requests may refer to the same
// synchronized application state, binders, handlers, or tags.
//
// [Container], [Tbody], [Select], [Option], and [Template] constructors return
// values; other constructors generally return pointers. Use Container, Tbody,
// Select, and Template as values because taking their addresses changes definition
// equality to pointer identity.
//
// See [jaws.UI] and each concrete widget for comparability, typed-nil, and
// zero-value behavior. No pointer widget in this package documents nil-receiver
// tolerance.
//
// Within one request, a widget normally backs one live [jaws.Element]. The
// HTML-inner widgets, [Img], [Option], [Template], [Container], [Tbody], and
// [Select] support multiple live Elements under their documented conditions.
// Input widgets and [JsVar] require distinct widget values.
//
// [RequestWriter.Register] is an advanced escape hatch primarily for binding a
// render-independent updater to otherwise static HTML authored by the surrounding
// template. The registered HTML is intended to contain no JaWS widgets. Render
// standard widgets normally; Register makes no compatibility guarantees for using
// them as its updater or inside its HTML.
//
// Container children must render one addressable direct DOM node carrying their
// Element's JaWS ID so reconciliation can remove and order them. [NewTemplate]
// supplies that node through its generated wrapper.
//
// HTML-inner widgets route content through [bind.MakeHTMLGetter]. Plain strings
// are treated as trusted HTML, while [bind.Getter][string], [bind.Binder][string]
// and [fmt.Stringer] values are escaped. Raw [template.HTMLAttr] params are also
// trusted and written as attributes as-is. Use getter/stringer forms or
// html/template escaping for untrusted user text.
package ui
