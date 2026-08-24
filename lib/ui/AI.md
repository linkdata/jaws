# AI guidance for github.com/linkdata/jaws/lib/ui

This is the canonical version-specific guide to JaWS widgets and template
helpers. Read the [module guidance](../../AI.md) first. Public behavior remains
documented on the exported symbols.

## Package role and building blocks

Package `ui` keeps widget rendering and browser-control behavior out of the root
request and session engine. Its primary building blocks are:

- `HTMLInner` for elements with dynamic inner HTML;
- `Input`, `InputText`, `InputBool`, and `InputDate` for typed control state;
- `Number` and `Range` for type-preserving numeric input;
- `Container`, `Tbody`, and `Select` for dynamic child lists;
- `Template`, `Handler`, `With`, and `RequestWriter` for template integration.

Use [bind](../bind/AI.md) for value adaptation, [tag](../tag/AI.md) for
dependency identity, [htmlio](../htmlio/AI.md) for low-level HTML output, and
[named](../named/AI.md) for Select and RadioGroup data.

## Widget lifetime, identity, and multiplicity

Every non-nil `jaws.UI` value must be comparable at runtime and equal to itself.
A statically comparable struct can still be unusable when an interface field
contains a slice, map, or function, or when it contains NaN. Invalid container
children cancel the Request with a cause matching `tag.ErrNotUsableAsTag`.

Every widget value is request-scoped. Construct fresh widgets for each Request,
usually through `RequestWriter`; never cache a widget and reuse it across
Requests. Distinct widgets may share synchronized application state, binders,
handlers, and tags.

Within one Request, a widget normally backs one live `jaws.Element`. The
following standard widgets support multiple live Elements under the conditions
documented on their concrete types:

- HTML-inner widgets, Img, and Option retain no Element-specific mutable state;
- Template, Container, Tbody, and Select keep that state in each Element's state
  slot rather than on the widget definition.

Input widgets and JsVar require distinct widget values. To show one binder in two
inputs, construct two widgets:

```go
binder := bind.New(&mu, &value)
left := ui.NewText(binder)
right := ui.NewText(binder)
```

Calling `rw.Text(binder)` twice or rendering `{{$.Text .Binder}}` twice performs
the same distinct construction.

Container, Tbody, Select, and Template constructors return values that must be
used as values; pointers to those definitions are unsupported. `NewOption` also
returns a value, but Option is stateless with value-receiver methods, so a
pointer remains a valid UI. It changes identity to pointer identity and is
usually unnecessary.

A nil UI interface is a render/update no-op. A typed nil is non-nil and is
dispatched to its concrete methods; its receiver behavior follows that type's
contract. Required operational collaborators follow the module nil convention.

## RequestWriter and templates

`RequestWriter` exposes helpers such as `Span`, `Text`, `Select`, `Container`,
`JsVar`, and `Template` for concise template use. Explicit construction is also
available through `rw.NewUI(ui.NewX(...), params...)`.

`rw.Template(outerTag, name, dot, params...)` renders a partial template inside
a generated addressable JaWS wrapper. An empty outer tag selects `div`; choose a
semantic wrapper such as `tr`, `td`, `li`, or `option` when DOM context requires
it. `NewTemplate(outerTag, name, dot, attrs...)` accepts trusted raw wrapper
attribute strings. Attribute precedence is render params, constructor attributes,
then the dot callback. Constructor attributes participate in Template equality.
When dot implements `jaws.InitialHTMLAttrHandler`, its callback supplies
per-Element attributes during initial render. The callback is not invoked during
`Template.JawsUpdate`. Full page templates belong in `ui.Handler`, which sets
`Cache-Control: no-store`. Custom page handlers must call `jw.NewRequest(w, r)`
before writing output;
`Request.HeadHTML` does not manage response headers. Static structural inclusion
should use Go's native template action:

```gotemplate
{{template "partial" .Dot}}
```

After creating each Request, `ui.Handler` checks the top-level Dot's method set,
including promoted methods, for `jaws.ConnectHandler` and installs `JawsConnect`
before page template execution. A plain GET only installs the callback; the
accepted WebSocket invokes it with the `jaws.ConnectFn` lifecycle. An
implementation available only on a nested Template Dot is ignored without a
diagnostic. Handler reuses its Dot across Requests, so its state and callbacks
must support concurrent execution. The bundled client connects after parsing
the document, while a custom client can dial once flushed response bytes expose
the request key and overlap initial rendering.

The Template's Dot contributes both identity and tags. It must be nil or
comparable at runtime, equal to itself, and usable under `tag.TagExpand`.
Implementing `JawsGetTag` does not repair a non-comparable Dot because tag
expansion and widget equality are separate constraints. Use `tag.Tag("name")`
instead of a plain string semantic tag.

A Template with a non-empty outer tag updates only an Element rendered by an
equal Template value. Template execution is not transactional: an error can
leave partial output, queued messages, or application side effects. A failed
attempt unregisters Elements it created; a failed update retains the previous
browser DOM and its Elements.

A Template owns every Element created through the RequestWriter passed to its
execution, including nested Template, Register, RadioGroup, and widget helpers.
A successful update unregisters the previous generation before replacing the
wrapper content. Call `$.RadioGroup` from the Template that renders the group;
ownership follows the call site, not the wrapper receiving the markup.

## Register escape hatch

`RequestWriter.Register` binds a render-independent `jaws.Updater` to otherwise
static HTML authored by the surrounding template. The returned Jid must be the
element's HTML `id`:

```gotemplate
<section id="{{$.Register .Dot.Panel}}" class="panel">
  template-authored content
</section>
```

Register never calls `JawsRender`. It uses the updater as a tag, attaches its
event handlers, applies tag and handler params, and calls `JawsUpdate` once.
Attribute params are ignored, so write attributes in the template.

The updater must be comparable, equal to itself, and usable as a tag. Reuse it
for multiple live Elements only when it retains no shared Element-specific state
and is concurrency-safe where required. A typed nil is invoked normally.

Registered HTML should contain no JaWS widgets. Render standard widgets through
their normal helpers; Register makes no compatibility guarantees for a standard
widget as its updater or for managed widgets nested in its HTML.

## HTML content and attributes

HTML-inner constructors and matching RequestWriter helpers route content through
`bind.MakeHTMLGetter`. The conversion precedence and trust boundary are owned by
the [bind guide](../bind/AI.md): plain strings and `template.HTML` are trusted,
while getter/stringer forms escape returned strings. String and
`template.HTMLAttr` render params, including slices, and `NewTemplate` attribute
strings are trusted raw attributes. Route untrusted content through escaping
getter/stringer forms. Build attributes from untrusted values with `htmlio.Attr`
and a trusted name; convert the result to `string` for `NewTemplate`.

`Element.ApplyGetter` registers a primary getter's tags and event interfaces; it
does not run initial-attribute hooks. Call `Element.ApplyInitialHTMLAttr`
separately and without holding a lock that the callback might acquire. A
`bind.Binder` acquires its own value lock before invoking its hook.

A Template without a wrapper does not invoke Dot's initial-attribute callback.
Wrapper attributes persist when `Template.JawsUpdate` replaces only the inner
HTML; change them with `Element.SetAttr` and `Element.RemoveAttr`.

Getter paths must not mutate domain state. They may queue wrapper changes with
Element update methods so class/attribute changes flush with the HTML update.
HTMLInner-backed widgets unconditionally queue `SetInner` when updated; input
widgets instead retain a baseline and suppress redundant `SetValue` operations.
Dirty only the output that actually changed.

## Event and browser boundaries

The bundled client forwards input, click, and context-menu events only while its
WebSocket is open and does not replay earlier interaction. When early input
matters, render controls disabled or make the region inert. In a custom page
handler, install a Request `ConnectFn` that updates synchronized request-local
readiness and dirties the request-specific readiness tag registered by the
gate, or the exact Element whose updater removes it. A reused `ui.Handler`
shares its Dot across Requests. Its `ConnectHandler` can validate the callback
Request or update synchronized shared state, but a scalar Dot field cannot serve
as a request-local readiness gate. Ordinary tag dirtying updates matching
Elements on every live Request.

Native form reset is unsupported for managed inputs and Select. A reset button
or `form.reset()` changes browser state without the per-control events JaWS
transports. Use a JaWS-handled `type="button"` action that updates authoritative
Go state and dirties the affected bindings.

Each independently constructed Radio is one boolean binding. Native grouping
unchecks peers without reporting them. Use `RequestWriter.RadioGroup` with a
single-select `named.BoolArray` of distinct names, or one synchronized mutation
that clears peers and dirties every changed binding.

Every browser-to-server WebSocket message must fit the 32 KiB inbound limit.
The client does not chunk input, JsVar, click, context-menu, or removal payloads.
An oversized message fails the WebSocket read and closes the Request connection.
The resulting read-limit error is retained in the Request cancellation cause,
which is passed to `Jaws.Log`; the message is not merely rejected for one
control. Use HTTP uploads for large values and smaller independently updated
wrappers for large trees. `JsVar.ClientCheck` runs after receipt and cannot
enforce this transport boundary. See [wire](../wire/AI.md).

## Input dirty targets

Writable sources used by Text, Password, Textarea, Checkbox, Radio, Number,
Range, and Date need a stable source-derived dirty target for post-event
reconciliation. `bind.New(&mu, &value)` provides the backing pointer. A custom
setter can be pointer-valued or implement `JawsGetTag`; that result takes
precedence over setter identity.

Tags passed as render params register dependencies but do not replace the
source-derived target. Editable Number and Range fail rendering without a usable
target. The reusable nonnumeric input bases render but cannot automatically
restore rejected or normalized browser values without one.

A custom setter containing a slice is not comparable, so expose its synchronized
backing pointer explicitly:

```go
type validatedText struct {
	bind.Setter[string]
	dirtyTag  *string
	forbidden []rune // immutable
}

func (s validatedText) JawsSet(elem *jaws.Element, value string) (err error) {
	for _, r := range value {
		if slices.Contains(s.forbidden, r) {
			err = errors.New("value contains a forbidden character")
			return
		}
	}
	err = s.Setter.JawsSet(elem, value)
	return
}

func (s validatedText) JawsGetTag() any { return s.dirtyTag }

func newUsernameInput(mu *sync.RWMutex, value *string) *ui.Text {
	return ui.NewText(validatedText{
		Setter:    bind.New(mu, value),
		dirtyTag:  value,
		forbidden: []rune{' ', '/', '\\'},
	})
}
```

After a set result other than `jaws.ErrValueUnchanged`, the input bases retain
and dirty their source target. The originating Element may also be dirtied
exactly when reconciliation must remain browser-local.

## Numeric inputs

Number and Range accept a `bind.Getter[T]` for any `Numeric` type: signed and
unsigned integers, `uintptr`, `float32`, `float64`, and named types with those
underlying types. A source becomes editable when it also implements
`bind.Setter[T]`.

Parsing, formatting, and setter calls retain T. Integers use base-10 syntax and
their actual width; floats use their actual bit size, including 32-bit precision
for float32. Non-finite values have no valid numeric input representation.

A getter-only Number renders readonly. A getter-only Range renders disabled and
is omitted from native form submission. Static numeric template values use these
getter-only forms.

Number sends edits on `change`, leaving pending text browser-local until then.
Range sends live `input` events. Both silently reject malformed or
unrepresentable text without calling the setter and restore canonical text only
on the originating control. Accepted input is reconciled with canonical
formatting even when the source reports an unchanged value.

Named numeric types work directly:

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

## JavaScript variables

JsVar binds a JSON-marshalable Go value to an application-owned property on
`window`. It is bidirectional: neither side becomes authoritative merely because
the binding exists. Create a fresh binding for each Request. A shared handler
can implement `JsVarMaker` so each render receives a new JsVar over synchronized,
possibly shared state.

```go
type application struct {
	clientMu sync.Mutex
	client   Client
}

func (app *application) JawsMakeJsVar(*jaws.Request) (ui.IsJsVar, error) {
	jsv := ui.NewJsVar(&app.clientMu, &app.client)
	jsv.ClientCheck = ui.JSONSizeCheck[Client](1 << 20)
	return jsv, nil
}

handler := ui.Handler(jw, "index", new(application))
```

```gotemplate
{{$.JsVar "client" .Dot}}
```

Several bindings may share a name. A browser write fans out to every live
binding of that name; a removed binding stops receiving it. If several bindings
share one non-idempotent backing value, that write is applied once per binding.

The server rejects the exact top-level name `__proto__`; the browser rejects that
exact component anywhere in a dotted `jawsVar` path. Names share the page global
namespace, so use an application-owned top-level symbol and dotted suffixes for
paths. Do not bind unrelated or browser-owned globals.

Rendering a non-nil Ptr serializes the current Go snapshot. The browser installs
it when the binding element attaches. Browser writes before the WebSocket opens
are not queued, and server broadcasts are not replayed to a rendered page that
has not subscribed. Applications needing convergence must define a handshake,
resend, merge, or browser-authoritative policy.

### JSON representation

JavaScript numbers cannot exactly represent integers outside
`-9007199254740991` through `9007199254740991`. Represent exact wide integers as
built-in strings and convert BigInt values back to strings before `jawsVar`. A Go
`json:",string"` tag is not generic round-trip support.

```js
const next = BigInt(client.counter) + 1n;
client.counter = next.toString();
jawsVar("client.counter", client.counter);
```

Generic browser writes decode into `any` and do not invoke destination custom
unmarshaling. Use a browser-facing DTO or implement `PathSetter` for types such
as `time.Time`, `[]byte`, and maps with non-string keys.

### ClientCheck and PathSetter

The generic setter can update exported JSON fields and grow slices and has no
default accumulated-state limit. `ClientCheck` validates the complete tentative
value before an actual generic write commits. It receives the browser-supplied
jq path unchanged. Empty components are ignored, so `.value.` aliases `value`;
treat the raw path as an inspection hint, not an authorization key. Array and
slice components must be canonical JavaScript array-index names representable
as Go `int`, and string-keyed map entries are exact. Struct path components and
map-to-struct keys follow `encoding/json`'s default field-selection rules. An
exact `json:"-"` tag excludes an otherwise selected exported field. For a
non-promoting field, a valid nonempty tag name is used verbatim, while an absent,
empty, or invalid name falls back to the Go field name; `json:"-,"` therefore
names the field `-`. Ambiguous fields are absent from the path namespace.

An anonymous struct without a valid explicit JSON name contributes its promoted
fields directly without a Go-type-name component: use `value`, not
`Inner.value`, or tag the anonymous field to create a nested path. Promotion
reaches exported fields through unexported embedded structs. An explicitly
named unexported anonymous struct is not itself a readable or writable endpoint
or writable map-to-struct key, but longer paths can reach its exported fields.
Reads and generic writes that traverse a nil pointer fail with
`jq.ErrPathNotFound`, and generic writes do not allocate it. `JawsGetPath`
returns nil on lookup failure, so nil does not distinguish failure from a
resolved nil value. Use `PathSetter` to allow-list paths and operations.

A check runs while the application locker is held. It must inspect only: do not
mutate or retain tentative state, re-enter the JsVar, call a path setter, acquire
the same locker, or return/wrap `jaws.ErrEventUnhandled`. A nil result commits;
an error rolls back without a broadcast. The browser already changed locally,
so an ordinary rejection can leave it divergent until application
resynchronization. A check using `jq.Get` cannot inspect an explicitly named
unexported anonymous struct at its own endpoint after a tentative write beneath
it; inspect a longer exported-field path or the Go value directly.

The check sees tentative Go state, not necessarily the decoded value later used
for a peer broadcast. jq conversions and ignored map-to-struct fields can make
them differ. Server writes, invalid or unchanged generic writes, and PathSetter
writes bypass ClientCheck.

`JSONSizeCheck[T]` marshals the whole tentative value. A non-positive limit
disables it. An over-limit value or marshaling failure matches
`ErrJsVarTooLarge` and cancels the associated Request after rollback. It bounds
encoded JSON, not Go heap/backing memory; custom marshalers, omitted fields,
aliases, and capacity may require a domain-specific check.

Configure equivalent policies and the same locker on every binding exposing the
same Ptr or reachable mutable state. One unchecked binding bypasses the policy.

Concurrent writes to one JsVar serialize, and resulting broadcasts retain that
order. Transport backpressure can delay later writes but does not hold the
application locker.

## Container-family widgets

`NewContainer`, `NewTbody`, and `NewSelect` return immutable definition values.
The provider or handler participates in equality and must itself be comparable
and reflexive. Keep application objects containing slices, maps, or functions
behind stable pointers and rebuild with the same pointer:

```go
rows := &RowCollection{/* synchronized state */}
first := ui.NewContainer("div", rows)
second := ui.NewContainer("div", rows) // equal to first
```

Equal rebuilt definitions let the same parent retain its Element. Equal values
can back several live Elements only when providers and reused children support
that multiplicity. Tbody embeds a Container fixed to `tbody`; replacing it is
unsupported. Select treats a nil-interface handler as a no-op; typed nils are
called normally.

Each child must render one addressable direct DOM node with its Element Jid.
`NewTemplate` supplies that wrapper. The slice returned by `JawsContains` becomes
read-only after return. Duplicate child values require a widget type that
supports multiple live Elements.

Reconciliation is parent-local and updates direct children only. Reordering
equal children retains Elements and complete nested subtrees. A changed nested
container needs its own dirty/update pass. Moving a definition between parents
does not preserve its Element.

## Element state and reconciliation

Container, Tbody, Select, and Template claim one private state slot on each
Element before callbacks, tag registration, or output. Contention returns
`jaws.ErrElementStateClaimed` without render side effects. Do not combine two
state-owning renderers on one Element.

Container state owns the render-time tag, reconciliation mutex, and children.
Widget definitions remain immutable. Provider callbacks and validation run
without the state mutex. Reconciliation holds it only while matching definitions
and creating Elements; rendering, removal, cancellation, recursive cleanup, and
logging occur after unlocking.

Cleanup detaches children under the state lock and recursively unregisters them
after unlocking. Failed render and append paths unregister every child and
nested owner they created. A successful Select render queues its selected value
after options; state contention suppresses reconciliation and that value update.

Template stores the Elements created by each execution in the rendering
Element's state. Equal Template values can therefore back multiple Elements and
be rebuilt by a container without losing identity. A composite updater must use
Template values equal under `==` for render and update.

## Authoring widgets

For a simple HTML widget, embed `HTMLInner`, adapt content with
`bind.MakeHTMLGetter`, apply the getter and initial attributes separately, and
write through `htmlio.WriteHTMLInner`:

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
```

For string, bool, and date controls, embed or compose the corresponding typed
input base. Number and Range are complete widgets rather than reusable numeric
bases because they own parsing, formatting, and event baselines.

For a container with only a distinct type and tag, embed a Container value. If
extra behavior is needed, keep it in a named field and delegate render and update
to that same value. The outer value must remain comparable/reflexive and must not
claim a second Element state slot.

Use a custom `JawsUpdate` only when behavior differs from rendering the original
getter again. Element SetAttr/RemoveAttr/SetClass/RemoveClass/SetInner/SetValue,
Append/Order/Remove/Replace operations belong only in render/update processing.

## Failures and tests

Container-family child render failures are application errors. Initial render
returns them. Update-time append failures go through `MustLog`; without a logger
that path may panic. A failed new child is removed from Request state and not
appended to the DOM so a later update can retry from fresh state. Other queued
steps are not rolled back.

Widget tests should use real Requests and Elements. Cover:

- event dispatch and `ErrEventUnhandled` fallthrough;
- exact dirty targets and shared dependency targets;
- accepted, rejected, unchanged, and canonically reformatted input;
- browser-open gating and native reset/radio boundaries;
- Template/Register ownership and stale-Element cleanup;
- equal container values on independent Elements;
- append, remove, order, nested subtree retention, and contention-before-callback;
- JsVar paths, validation, rollback, shared names, ordering, precision, and size;
- both race/debug and plain production builds.

Changes to `int` or `uint` numeric bounds also require the 32-bit leg in the
[repository verification matrix](../../AI.md#repository-verification-matrix).

Performance changes require committed benchmarks aimed at the actual cost. Use
parallel benchmarks for contention and `ReportAllocs` for per-operation paths.
