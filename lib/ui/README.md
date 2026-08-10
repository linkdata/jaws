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
own JaWS identity and wrapper-level attributes. Passing an empty tag selects the
default `div` wrapper. Attribute params passed to `rw.Template(...)` are applied
to that generated wrapper.
Template bodies used with `rw.Template(...)` must be partials; full page
templates should be rendered through `ui.Handler`.

`rw.Register(...)` is an advanced escape hatch primarily for attaching a
render-independent updater to otherwise static HTML written directly in the
surrounding template. The registered HTML is intended to contain no JaWS widgets.
Its returned JaWS ID must become that element's `id`:

```gotemplate
<section id="{{$.Register .Dot.Panel}}" class="panel">
  template-authored content
</section>
```

`Register` never calls `JawsRender`; use it for a custom updater designed to work
without render-time initialization. It tags the element with the updater, attaches
its event handlers, and calls `JawsUpdate` once for initial state. Write HTML
attributes in the template because attribute params are ignored. Render standard
widgets normally. `Register` makes no compatibility guarantees for using a
standard widget as its updater or placing JaWS widgets inside its HTML; behavior
in either position is unspecified.

Use Go's native template action when a static structural fragment must be included
without another JaWS-managed wrapper:

```gotemplate
{{template "partial" .Dot}}
```

`rw.Template` supplies an addressable wrapper for updates and container
reconciliation. Choose the semantic element required by the DOM context, such as
`tr`, `td`, `li`, or `option`, instead of relying on the `div` default there.

Template execution is not transactional. An error may leave partial output, queued
messages, or application side effects in place. Elements created by the failed
attempt are unregistered; a failed update retains the previous browser DOM and its
Elements.

A Template owns every Element created through its `RequestWriter`. A successful
update unregisters the previous generation when it replaces the wrapper content.

Call `$.RadioGroup` from the Template that renders the group; ownership follows the
call site rather than the wrapper receiving its markup.

Use the `ui.Template` value returned by `NewTemplate` directly; taking its address
changes container identity to pointer identity. A container may retain Elements for
equal values rebuilt by `JawsContains`. A Template must be comparable and equal to
itself, and its `Dot` must be nil or usable as a tag under `tag.TagExpand`. Use
`tag.Tag("...")` for string tags.

A Template with a non-empty `OuterHTMLTag` can update only an Element rendered by
an equal Template value.

## Container-family value widgets

`NewContainer`, `NewTbody`, and `NewSelect` return definition values. Treat them as
immutable after use and do not take their addresses; pointers change definition
equality to pointer identity. `Tbody` embeds a `Container` configured for `tbody`;
replacing it is unsupported.

The provider or handler is part of the value's identity. Its dynamic value must be
comparable at runtime and equal to itself. Keep application objects containing
slices, maps, or functions behind stable pointers, and rebuild with the same
pointer:

```go
rows := &RowCollection{/* synchronized application state */}
first := ui.NewContainer("div", rows)
second := ui.NewContainer("div", rows) // equal to first
```

Rebuilding an equal definition lets the same parent retain its live Element. Equal
values may back several live Elements within one request when their providers or
handlers are safe for all calls and each child UI value reused across those Elements
supports multiple live Elements.

Each child must render one addressable direct DOM node carrying its Element's JaWS ID,
because removal and ordering target that node. Construct Template children with
`NewTemplate`, which always supplies a wrapper.

Reconciliation updates direct children only. Reordering retained equal children
preserves their Elements and nested subtrees; changed nested containers need their own
update. Child Element identity is parent-scoped, so moving a child definition between
parents does not preserve its Element.

The zero Container and Tbody, and values constructed with a nil-interface child
provider, panic when rendering or updating calls the missing provider. A zero
Select behaves the same for render and update, while its `JawsInput` is a no-op.
A typed-nil provider is called normally and must tolerate its nil receiver itself.

You can also use explicit constructors through:

```go
rw.NewUI(ui.NewX(...), params...)
```

Examples:

```go
rw.NewUI(ui.NewDiv("content"))
rw.NewUI(ui.NewCheckbox(myBoolSetter), "disabled")
rw.NewUI(ui.NewRange(myIntSetter), `min="0"`, `max="100"`)
```

HTML-inner widgets such as `NewDiv`, `NewSpan`, and `RequestWriter.Div` pass
their content through `bind.MakeHTMLGetter`. Plain strings are treated as trusted
HTML and are not escaped; use a `bind.Getter[string]`, `bind.StringGetterFunc`,
or `fmt.Stringer` for string content that should be escaped.

## Numeric inputs

`NewNumber` and `NewRange` accept a `bind.Getter[T]` for any `Numeric` type:
signed and unsigned integers, `uintptr`, `float32`, `float64`, and named types
with one of those underlying types. The source is editable when its dynamic type
also implements `bind.Setter[T]`.

The Go binding remains `T` throughout parsing, formatting, and setter calls.
Integers use base-10 integer syntax and enforce their actual bit width. Floats
parse and format at their own bit size; `float32` uses 32-bit precision. Non-finite
values have no valid numeric input representation.

An editable source must expose at least one stable, usable source tag when it is
rendered. Pointer-valued sources are usable directly; a source can instead implement
`JawsGetTag`. `bind.New(&mu, &value)` supplies the backing value pointer as its tag.
A changing getter-only source also needs a tag for dirty-driven server updates.

A getter-only Number has the native `readonly` attribute. A getter-only Range is
`disabled` and is therefore omitted from native form submission. Static numeric
values passed to `RequestWriter.Number` and `RequestWriter.Range` use these
getter-only paths.

Number sends edits when the browser fires `change`. Pending edits remain
browser-local until then. Range sends live `input` events while its thumb moves.
Both silently reject malformed or unrepresentable browser text without calling the
setter and restore the getter's canonical text only on the originating control.
Accepted text is reconciled through the formatter when needed, including when the
source value is unchanged.

Named numeric sources work directly with both Go and template helpers:

```go
type Percent uint8

var mu sync.RWMutex
percent := Percent(50)
binder := bind.New(&mu, &percent)

number := ui.NewNumber(binder)
slider := ui.NewRange(binder)
```

```gotemplate
{{$.Number .Dot.Percent `step="1"`}}
{{$.Range .Dot.Percent `min="0"` `max="100"` `step="1"`}}
```

`JsVar` values are client-writable. The generic path setter can write exported
JSON fields and append to slices, and it has no default accumulated-state size
limit. Set the binding's optional `ClientCheck` to validate each actual generic
browser change before it commits. The check receives the complete tentative
value and browser-supplied jq path. The path is passed through unchanged and
may be noncanonical, so use it as an inspection hint rather than an
authorization key; use `PathSetter` to allow-list paths. A rejected check rolls
the tentative change back without broadcasting it. Ordinary check errors do
not cancel an associated request. Checks must not return or wrap
`jaws.ErrEventUnhandled`, which has event-handler fallthrough semantics.
The check validates tentative Go state, not the decoded browser value carried
by an accepted peer broadcast; jq conversions or ignored map-to-struct entries
can make them differ.

`JSONSizeCheck[T](maxBytes)` is a ready-made exact size policy. It marshals the
complete tentative value for every actual generic browser change, so its work is
determined by the whole value and its marshaling behavior; map-key sorting and
custom marshalers can add further cost. A non-positive limit disables it. An
over-limit tentative value, or one that cannot be marshaled, returns
`ErrJsVarTooLarge` and cancels the associated request, when present, after
rollback. `ClientCheck` is not called for server-initiated writes or values
implementing `PathSetter`. Use `PathSetter` to allow-list paths or collection
operations and enforce its bounds there.
Configure an equivalent `ClientCheck` and the same locker on every
request-scoped binding that exposes the same `Ptr` or reachable mutable backing
state to browser writes. `JSONSizeCheck` bounds `encoding/json` output, not Go
heap or backing-memory size; custom `MarshalJSON` or `MarshalText` methods,
omitted fields, aliases, or collection capacity require a domain-specific
check.

`ClientCheck` is an acceptance gate, not a monitor: initial render, server
writes, invalid or unchanged generic writes, and `PathSetter` writes bypass it.
An ordinary rejection rolls Go state back without a broadcast, so the
originating browser can remain divergent until the application resynchronizes
it. An `ErrJsVarTooLarge` rejection terminates the associated request
connection, when present.

Concurrent writes to one `JsVar` are applied one at a time, and any broadcasts
they produce preserve that order. Transport backpressure can delay later writes,
but it does not keep the locker passed to `NewJsVar` held.

## Building blocks

- `HTMLInner`
  - For tags like `<div>...</div>`, `<span>...</span>`, `<td>...</td>`.
- `Input`, `InputText`, `InputBool`, `InputDate`
  - For interactive inputs with typed parse/update behavior.
- `Container`, `Tbody`, `Select`
  - Definition value widgets for rendering and maintaining dynamic child lists.

## Widget lifetime

Every UI widget value is request-scoped. Construct a fresh widget for each
request, typically through `RequestWriter` helpers such as `$.Span(...)`,
`$.Text(...)`, `$.Container(...)`, and `$.JsVar(...)`. Do not cache a widget and
reuse it across requests, even if that widget currently appears stateless.

Within a request, a widget normally backs at most one live `jaws.Element`. A
widget may back multiple live Elements only when its type documents that
support. Stateless HTML widgets and Template support this directly; Container,
Tbody, and Select keep mutable state on each Element instead of on the shared
value. The [package documentation](doc.go) is the canonical standard-widget
classification; each concrete type's Go documentation states its conditions.

The application data referenced by widgets has a separate lifetime. Distinct
request-scoped widgets may share synchronized backing state, binders, handlers
and tags. For `JsVar`, use a `JsVarMaker` when a shared handler or template value
needs to create the binding for the current request.

For example, render one bound value in two text inputs by constructing two
widgets over the same binder:

```go
binder := bind.New(&mu, &value)
left := ui.NewText(binder)
right := ui.NewText(binder)
```

Direct construction is not required: calling `rw.Text(binder)` twice constructs
the two distinct widgets automatically. In a template, render
`{{$.Text .Binder}}` twice for the same result.

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
  e.ApplyGetter(w.HTMLGetter)
  getterAttrs := e.ApplyInitialHTMLAttr(w.HTMLGetter)
  attrs := append(e.ApplyParams(params), getterAttrs...)
  return htmlio.WriteHTMLInner(wr, e.Jid(), "article", "", w.HTMLGetter.JawsGetHTML(e), attrs...)
}

// JawsUpdate is inherited from the embedded ui.HTMLInner.
```

`ApplyGetter` registers the getter's tag and event handlers;
`ApplyInitialHTMLAttr` runs the getter's `JawsInitialHTMLAttr` callback and must
be called without holding any lock that callback may acquire.

## Adding an interactive input widget

Use one of the typed input bases:

- `InputText` for string-based inputs
- `InputBool` for boolean inputs
- `InputDate` for `time.Time` inputs

Number and Range own their type-preserving numeric parsing, formatting, and event
state; they are complete widgets rather than reusable input bases. The `Numeric`
constraint defines their supported Go types.

Each base handles:

- tracking last rendered value
- receiving `what.Input`
- retaining and dirtying the setter-derived tag after any set result other than
  `jaws.ErrValueUnchanged`; see
  [`Input`](https://pkg.go.dev/github.com/linkdata/jaws/lib/ui#Input)
- update-driven `SetValue` pushes

## Adding a container widget

Embed a Container value when the new widget only supplies a distinct Go type and
HTML tag:

```go
type UList struct{ ui.Container }

func NewUList(c jaws.Container) UList {
  return UList{Container: ui.NewContainer("ul", c)}
}
```

Embedding promotes Container's render and update methods. A widget that needs to
add behavior can keep the Container in a named field and delegate both methods to
that same value:

```go
type UList struct {
  container ui.Container
}

func NewUList(c jaws.Container) UList {
  return UList{container: ui.NewContainer("ul", c)}
}

func (w UList) JawsRender(e *jaws.Element, wr io.Writer, params []any) error {
  return w.container.JawsRender(e, wr, params)
}

func (w UList) JawsUpdate(e *jaws.Element) {
  w.container.JawsUpdate(e)
}
```

The outer widget must itself be comparable and equal to itself. It must also
respect the one-state-slot rule: do not claim Element state in the outer widget
when the delegated Container claims it.

## Container error behavior

Container, Tbody, and Select treat child render failures as application bugs.

- During initial render, child render failures are returned as errors.
- During updates, append render failures are reported through `MustLog` (and
  may panic if no logger is configured).
- A newly appended child that fails to render is dropped from request state and
  not appended to the browser DOM, so later updates can retry it from fresh
  state. Other already-queued update steps are not rolled back.
