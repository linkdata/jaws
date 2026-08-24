// Package ui contains the standard JaWS widgets and template helpers.
//
// Its main building blocks are [HTMLInner] for dynamic inner HTML; [Input],
// [InputText], [InputBool], and [InputDate] for typed controls; [Number] and
// [Range] for numeric controls; [Container], [Tbody], and [Select] for dynamic
// children; and [Template], [Handler], and [RequestWriter] for template integration.
//
// Every non-nil value used as a [github.com/linkdata/jaws.UI] must be comparable
// at runtime and equal to itself, and is scoped to one Request. Construct fresh
// widgets for each Request; they may share synchronized application state,
// binders, handlers, and tags.
//
// Within one Request, a widget normally backs one live
// [github.com/linkdata/jaws.Element]. Widgets based on [HTMLInner], plus [Img],
// [Option], [Template], [Container], [Tbody], and [Select], support multiple live
// Elements under their concrete contracts. Input widgets and [JsVar] require
// distinct widget values.
//
// [NewContainer], [NewTbody], [NewSelect], and [NewTemplate] return definition
// values. Use them as values; taking their addresses replaces definition equality
// with pointer identity and is unsupported.
//
// HTML-inner widgets route content through
// [github.com/linkdata/jaws/lib/bind.MakeHTMLGetter]. Existing
// [github.com/linkdata/jaws/lib/bind.HTMLGetter] values are used unchanged, and
// plain strings and [html/template.HTML] are trusted HTML. Adapters for string-valued
// [github.com/linkdata/jaws/lib/bind.Getter] and
// [github.com/linkdata/jaws/lib/bind.Binder] values and [fmt.Stringer] output are
// escaped. Raw [html/template.HTMLAttr] parameters and [NewTemplate] attribute
// strings are also trusted. Escape untrusted text before it reaches a trusted form.
//
// Browser input, click, and context-menu events are forwarded only while the
// WebSocket is open and are not replayed. Native form reset does not update Go
// bindings, and independently bound [Radio] values do not become one server-side
// group by sharing an HTML name; see [RequestWriter.RadioGroup].
//
// Each browser-to-server WebSocket message is limited to 32 KiB by
// [github.com/linkdata/jaws.Request.ServeHTTP]. Standard widgets do not chunk
// payloads. An oversized message closes the connection, and its read-limit error
// is retained in the Request cancellation cause, which is passed to
// [github.com/linkdata/jaws.Jaws.Log].
package ui
