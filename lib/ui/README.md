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

Use Go's native template action when a static structural fragment must be included
without another JaWS-managed wrapper:

```gotemplate
{{template "partial" .}}
```

JaWS-managed partials need one addressable direct DOM node for updates and
container reconciliation. Choose the semantic wrapper required by the DOM context,
such as `tr`, `td`, `li`, or `option`, instead of relying on the `div` default there.

Template execution is best-effort rather than transactional. Nested UI helpers
such as `{{$.Span ...}}` register elements as the template runs, and custom
template actions may queue updates or mutate application state. If execution
later returns an error, JaWS returns or logs that error and preserves whatever
already happened; it does not roll back partial output, queued messages, or
application side effects. The tracked elements the failed execution registered are
unregistered, since nothing will update them.

A template owns every element created through the `RequestWriter` it is given —
`{{$.Span ...}}`, `{{$.Button ...}}`, `{{$.Register ...}}`, `{{$.RadioGroup ...}}`,
a nested `{{$.Template ...}}`, and so on. A successful update unregisters the ones
the previous render left behind, along with the DOM that `SetInner` replaces.
Ownership is recorded when an element is created rather than after it renders, so
an element that never reached the browser is reclaimed too. On updates that
`SetInner` is queued only after a complete successful render, so a failed update
leaves the browser DOM unchanged — and with it the previous render's elements —
while earlier server-side side effects from that attempted render may remain. Treat
template execution errors as application bugs: validate data before rendering and
keep template actions infallible once they start emitting output or nested UI.

`$.RadioGroup` has one attribution condition: its radio and label elements belong to
the template whose body called it, not to the wrapper their markup lands in. Call it
from the template that renders the group; see `RequestWriter.RadioGroup` for what
happens when the two differ.

The ownership set lives in the element's widget state slot (`jaws.SetElementState`),
not on the `ui.Template` value, which is what keeps `NewTemplate` returning a plain
value. That matters for containers: a `JawsContains` implementation may
rebuild equal child values on every call and the container will still reuse their
elements, because that equality *is* the reuse key. Always use `ui.Template` as a
value, as `NewTemplate` returns it; taking its address is unsupported because it changes
container reuse to pointer identity. Under the general `jaws.UI` contract, the resulting
Template must be comparable at runtime and equal to itself — a slice, map or func `Dot`
makes the whole widget unusable, and implementing `tag.TagGetter` does not change that,
since it addresses tag resolution rather than widget comparability.

Comparability alone is not enough. A nil-interface `Dot` is valid and contributes no tag.
A typed nil is a non-nil interface and follows its dynamic type's comparability and
expansion rules. Rendering expands a non-nil-interface `Dot` through `tag.TagExpand`,
which rejects the exact dynamic types `string`, `bool`, `int`/`int8`/`int16`/`int32`/`int64`,
`uint`/`uint8`/`uint16`/`uint32`/`uint64`, `float32`/`float64`, `template.HTML`,
`template.HTMLAttr`, `jid.Jid` and `key.Key`. Aliases of a rejected type have that same
dynamic type and are rejected. `uintptr` and the complex types are not on the rejection
list. Other defined types are not rejected merely because their underlying predeclared
type is on it; they must still be comparable and equal to themselves. Wrap a rejected
scalar in `tag.Tag("...")` or a comparable struct when it should be a tag.

A template claims that slot while rendering, so at most one template may render a given
element. A composite UI must use template values equal under `==` for rendering and
updating that element; using unequal values is unsupported. A Template is not usable as a
`$.Register` updater — `$.Register` never invokes its updater's render method, so no
wrapper state exists to reconcile. On an element no template claimed, a Template returned
by `NewTemplate` reports `ErrElementStateUnclaimed` through `jaws.Request.MustLog`, which
**panics** when no `Jaws.Logger` is configured.

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

Container, Tbody, and Select support update-only use through `Register`. A Select
registered this way retains no handler-derived tag for its own post-set dirtying.
Any handler-initiated dirtying still occurs, and separately registered tags remain
registered. Use ordinary Select rendering when Select should register and use a usable
tag exposed by its handler.

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
- `Input`, `InputText`, `InputBool`, `InputFloat`, `InputDate`
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
  _, getterAttrs := e.ApplyGetter(w.HTMLGetter)
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
