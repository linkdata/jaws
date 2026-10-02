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

Input widgets and JsVarStore bindings require distinct widget values. To show
one binder in two inputs, construct two widgets:

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
`Template` for concise template use. Explicit construction is also
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
the document.

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

`ui.New(inner)` uses the adapted inner value for HTML and tag contributions, but
does not inherit its event or initial-attribute methods. Add those to the
returned Object. A direct `HTMLGetter` without `JawsGetTag` loses its implicit
tag when wrapped in `ui.New`.

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
wrappers for large trees. `JsVarStore.ClientCheck` runs after receipt and cannot
enforce this transport boundary. See [wire](../wire/AI.md).

## Input dirty targets

Writable sources used by Text, Password, Textarea, Checkbox, Radio, Number,
Range, and Date need a stable source-derived dirty target for post-event
reconciliation. `bind.New(&mu, &value)` provides the backing pointer. A custom
setter can be pointer-valued or implement `JawsGetTag`; that result takes
precedence over setter identity.

Tags passed as render params register dependencies but do not replace the
source-derived target. Editable Number and Range fail rendering without a usable
target. `InputText`, `InputBool`, and `InputDate` record their setter-derived
target when `RenderInput` renders an input. Their promoted `JawsInput` methods
use that target to reconcile rejected or normalized browser values. Without a
usable setter-derived target, automatic reconciliation does not occur.

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

JsVarStore owns one application value and one browser name. Its Go value is
authoritative. Create one store for shared state, then call Bind once per
Request during initial page rendering on the Jaws instance passed to
NewJsVarStore. Keep the binding outside regions that may be replaced or removed.
The browser reads and writes the live path from `window`. For dotted names,
the parent object must exist when jaws.js attaches. Top-level names can be
created by the binding. Application globals must be `window` properties;
browser and third-party globals may also be bound.
Each Request may bind a browser path only once; overlapping names such as
`client` and `client.x` conflict.
Initial data and patches assign to the live path, so a browser setter may run.

```go
store, err := ui.NewJsVarStore(jw, "client", &mu, &client)
if err != nil {
	return err
}
store.ClientCheck = func(source *jaws.Element, next *Client, path string) error {
	return validateClient(source, next, path)
}
```

Assign `store` to a `ClientStore *ui.JsVarStore[Client]` field on the page Dot,
then render a binding in the initial page template:

```gotemplate
{{$.NewUI (.Dot.ClientStore.Bind)}}
```

ClientCheck is required for browser writes; nil denies them. Each changed
proposal is tentative until ClientCheck accepts the complete value under the
store lock. An error or panic rolls it back. The check must only inspect: it
must not acquire the same lock, mutate or retain tentative data, or call a store
setter. The source Element can authorize a user or session. Every binding sees
the same JSON value, so use separate stores for data with different visibility.
Root or parent proposals can change multiple fields, including Go fields omitted
from JSON; validate the complete value, not only the path.

`jawsVar("client.x", value)` sends one proposal when connected, then assigns the
live variable and returns true if assignment succeeds. A one-argument call
reads the live value and attempts to propose it when bound and connected. This
also sends direct browser-side mutations. Unbound paths work locally; a false
write leaves the local value alone. Accepted changes update every binding.
Rejected, invalid, or unchanged proposals correct the source binding. A size
rejection cancels its Request for reload recovery.
Server writes use SetPath or DeletePath. WriteLocked groups path edits under
one lock; ReadLocked borrows the value under a read lock. Neither callback may
retain mutable borrowed data or re-enter a lock-taking method.

The empty path replaces the root; dotted paths have nonempty components.
Names and components named __proto__, constructor, or prototype are reserved.
Server paths are application-controlled; browser proposal paths are untrusted
and must be authorized by ClientCheck. Paths are limited to 4096 UTF-8 bytes.
Browser proposals replace existing paths in the encoded JSON, so they cannot
append slice elements one message at a time. Plain JSON trees have matching Go
and encoded paths; for complex shapes, non-root proposals are checked against
the encoded value and may change only their visible subtree. A Go field tagged
json:"value" is addressed as value, not Value. JSON null is a value; DeletePath
removes a string-keyed map entry.

The store records changed paths under its value lock and dirties its tag. Each
binding reads changes since its own rendered version when its Request updates,
including changes accumulated while the WebSocket was pending. The log keeps at
most 64 paths and 16 KiB of path bytes; a binding behind it receives a root
patch. The binding extracts partial JSON from one current root encoding for
ordinary JSON trees. Custom marshalers, promoted fields, slices, dynamic
interfaces, and other complex shapes use root patches. Map trees receiving
partial patches must have no shared mutable aliases between separately
addressable paths; changing one aliased map can change another JSON path.
Every bound value must remain JSON encodable with unique object member names.
Changed browser proposals are checked for encodability; JSONSizeCheck can also
bound their encoded size. ExtraTags can dirty derived UI after changed writes.

JavaScript numbers cannot exactly represent integers outside
-9007199254740991 through 9007199254740991. Use built-in string fields and
explicit BigInt conversion for exact wide integers. A Go json:",string" tag
changes the outbound representation, but generic browser writes still use
jq conversion rather than destination custom unmarshaling. JSONSizeCheck limits
encoded bytes, not Go heap capacity. An over-limit proposal cancels its Request.

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
unsupported. `Select.JawsInput` ignores a nil-interface handler; render and
update require one. Typed nils are called normally.

`Select` accepts one selected option. When using `named.BoolArray` as its
handler, construct it with `named.NewBoolArray(false)` or use the zero value.
Do not pass the HTML `multiple` attribute or a multi-select `BoolArray`.

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

Updating a Container, Tbody, or Select Element that has not been rendered logs
`ui.ErrElementStateUnclaimed` without calling its provider or queuing work.

Container state owns the render-time tag, reconciliation mutex, and children.
Widget definitions remain immutable. Provider callbacks and validation run
without the state mutex. Reconciliation holds it only while matching definitions
and creating Elements; rendering, removal, cancellation, recursive cleanup, and
logging occur after unlocking.

Cleanup detaches children under the state lock and recursively unregisters them
after unlocking. Failed render and append paths unregister every child and
nested owner they created. A successful Select render queues its selected value
after options; unusable state suppresses reconciliation and that value update.

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

For a custom HTML input, embed the matching string, bool, or date base and call
its `RenderInput` method from `JawsRender`. The base supplies `JawsInput` and
`JawsUpdate`:

```go
type Email struct{ ui.InputText }

func (u *Email) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return u.RenderInput(e, w, "email", params...)
}
```

Construct it with `&Email{InputText: ui.InputText{Setter: bind.New(&mu, &value)}}`.
`RenderInput` emits an `<input>` element; `Textarea` has its own render path.
Number and Range are complete widgets rather than reusable numeric bases
because they own parsing, formatting, and event baselines.

For a container with only a distinct type and tag, embed a Container value. If
extra behavior is needed, keep it in a named field and delegate render and update
to that same value. The outer value must remain comparable/reflexive and must not
claim a second Element state slot.

An outer UI may embed a standard widget and override `JawsRender` or `JawsUpdate`
when one phase genuinely requires behavior the standard widget cannot express.
Retain and delegate to the embedded widget for the standard phase; the outer UI
remains the Element's definition and is responsible for preserving the embedded
widget's registration, ownership, and multiplicity contracts.

Use a custom `JawsUpdate` only when behavior differs from reading the original
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
- JsVarStore paths, validation, rollback, name conflicts, ordering, precision, and size;
- both race/debug and plain production builds.

Changes to `int` or `uint` numeric bounds also require the 32-bit leg in the
[repository verification matrix](../../AI.md#repository-verification-matrix).

Performance changes require committed benchmarks aimed at the actual cost. Use
parallel benchmarks for contention and `ReportAllocs` for per-operation paths.
