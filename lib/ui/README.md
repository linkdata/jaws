# `github.com/linkdata/jaws/lib/ui`

This package is the home of JaWS widget implementations.

## Goals

- Keep widget logic out of JaWS core request/session internals.
- Make new widget authoring local to this package.
- Provide short widget naming (`ui.Span`, `ui.NewSpan`).
- Expose template context types (`ui.RequestWriter`, `ui.With`).

### RequestWriter helper calls

`ui.RequestWriter` exposes helper methods like `rw.Span(...)`,
`rw.Text(...)`, and `rw.Select(...)` for concise template use.
`rw.Template(tag, ...)` renders partial templates inside a generated JaWS
wrapper using the provided HTML tag, so template bodies should let that wrapper
own JaWS identity and wrapper-level attributes. Passing an empty tag renders the
template without a generated wrapper. Attribute params passed to
`rw.Template(...)` are applied to the generated wrapper when one exists.
Template bodies used with `rw.Template(...)` must be partials; full page
templates should be rendered through `ui.Handler`.

Template execution is best-effort rather than transactional. Nested UI helpers
such as `{{$.Span ...}}` register elements as the template runs, and custom
template actions may queue updates or mutate application state. If execution
later returns an error, JaWS returns or logs that error and preserves whatever
already happened; it does not roll back partial output, nested elements, queued
messages, or application side effects. On updates, the wrapper's `SetInner` is
queued only after a complete successful render, so a failed update leaves the
browser DOM unchanged while earlier server-side side effects from that attempted
render may remain. Treat template execution errors as application bugs: validate
data before rendering and keep template actions infallible once they start
emitting output or nested UI.

You can also use explicit constructors through:

```go
rw.NewUI(ui.NewX(...), params...)
```

Examples:

```go
rw.NewUI(ui.NewDiv("content"))
rw.NewUI(ui.NewCheckbox(myBoolSetter), "disabled")
rw.NewUI(ui.NewRange(myFloatSetter))
```

HTML-inner widgets such as `NewDiv`, `NewSpan`, and `RequestWriter.Div` pass
their content through `bind.MakeHTMLGetter`. Plain strings are treated as trusted
HTML and are not escaped; use a `bind.Getter[string]`, `bind.StringGetterFunc`,
or `fmt.Stringer` for string content that should be escaped.

`JsVar` values are client-writable. If a browser must only write selected fields
or bounded collections, make the bound value implement `PathSetter` and validate
allowed paths there. The generic path setter is convenient for trusted demos, but
it can write exported JSON fields and append to slices until the serialized
`MaxClientJsVarBytes` cap is hit on render.

Concurrent writes to one `JsVar` are applied one at a time, and any broadcasts
they produce preserve that order. Transport backpressure can delay later writes,
but it does not keep the locker passed to `NewJsVar` held.

## Building blocks

- `HTMLInner`
  - For tags like `<div>...</div>`, `<span>...</span>`, `<td>...</td>`.
- `Input`, `InputText`, `InputBool`, `InputFloat`, `InputDate`
  - For interactive inputs with typed parse/update behavior.
- `ContainerHelper`
  - For widgets that render and maintain dynamic child lists.

## Widget lifetime

Every UI widget value is request-scoped. Construct a fresh widget for each
request, typically through `RequestWriter` helpers such as `$.Span(...)`,
`$.Text(...)`, `$.Container(...)`, and `$.JsVar(...)`. Do not cache a widget and
reuse it across requests, even if that widget currently appears stateless.

The application data referenced by widgets has a separate lifetime. Distinct
request-scoped widgets may share synchronized backing state, binders, handlers
and tags. For `JsVar`, use a `JsVarMaker` when a shared handler or template value
needs to create the binding for the current request.

## Adding a simple static widget

Embed `HTMLInner` for the update behavior and render with the exported
`htmlio.WriteHTMLInner` (the package-internal widgets use an equivalent private
helper, which is not accessible from outside `package ui`):

```go
type Article struct{ ui.HTMLInner }

func NewArticle(inner any) *Article {
  return &Article{HTMLInner: ui.HTMLInner{HTMLGetter: bind.MakeHTMLGetter(inner)}}
}

func (w *Article) JawsRender(e *jaws.Element, wr io.Writer, params []any) error {
  _, getterAttrs, err := e.ApplyGetter(w.HTMLGetter)
  if err != nil {
    return err
  }
  attrs := append(e.ApplyParams(params), getterAttrs...)
  return htmlio.WriteHTMLInner(wr, e.Jid(), "article", "", w.HTMLGetter.JawsGetHTML(e), attrs...)
}

// JawsUpdate is inherited from the embedded ui.HTMLInner.
```

## Adding an interactive input widget

Use one of the typed input bases:

- `InputText` for string-based inputs
- `InputBool` for boolean inputs
- `InputFloat` for numeric inputs
- `InputDate` for `time.Time` inputs

Each base handles:

- tracking last rendered value
- receiving `what.Input`
- applying dirty tags on successful set
- update-driven `SetValue` pushes

## Adding a container widget

Use `ContainerHelper`:

```go
type UList struct{ ui.ContainerHelper }

func NewUList(c jaws.Container) *UList {
  return &UList{ContainerHelper: ui.NewContainerHelper(c)}
}

func (w *UList) JawsRender(e *jaws.Element, wr io.Writer, params []any) error {
  return w.RenderContainer(e, wr, "ul", params)
}

func (w *UList) JawsUpdate(e *jaws.Element) {
  w.UpdateContainer(e)
}
```

## Container error behavior

`ContainerHelper` treats child render/update failures as application bugs.

- During initial render, child render failures are returned as errors.
- During updates, append render failures are reported through `MustLog` (and
  may panic if no logger is configured).
- A newly appended child that fails to render is dropped from request state and
  not appended to the browser DOM, so later updates can retry it from fresh
  state. Other already-queued update steps are not rolled back.
