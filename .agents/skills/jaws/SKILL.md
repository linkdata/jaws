---
name: jaws
description: Build or refactor Go UIs with github.com/linkdata/jaws. Use for page and partial rendering, containers, binders, events, dependency tags, sessions, or updates.
metadata:
  short-description: Build apps with JaWS
---

# JaWS application design

Build the UI directly from synchronized server state. JaWS is an immediate-mode,
server-driven UI framework, not MVC. Authoritative server state is the only
application state model; this is a design requirement, not a preference.

## Match the application version first

Resolve the JaWS source selected by the consumer module, for example with:

```sh
go list -m -f '{{.Dir}}' github.com/linkdata/jaws
```

Use that directory, including any active `replace`; never substitute a sibling
checkout or guidance from another version.

Read only the version-matched guides relevant to the task:

- `lib/ui/AI.md` for handlers, templates, widgets, containers, or JsVar.
- `lib/bind/AI.md` for binders, getters, setters, conversion, or hooks.
- `lib/tag/AI.md` for choosing, registering, expanding, or debugging dependency
  keys.
- Root `AI.md` for Requests, Elements, dirty dispatch, serving, sessions,
  routing, context, or transport behavior.
- `examples/AI.md` and one relevant example guide when producing example code.

When changing JaWS itself, also follow its repository rule to read the root
guide and the guide beside every package changed. If an installed dependency
does not contain a guide, inspect its exported docs and source instead.

## Immediate-mode ownership model

- Application code reconstructs desired UI definition values from synchronized
  server state.
- JaWS retains live Elements and the original definitions attached to them for
  events, updates, and reconciliation.
- Construct fresh UI definitions for each Request. Definitions may refer to
  synchronized shared application state, binders, handlers, and stable tags.
- Do not retain JaWS Requests, Elements, or UI definitions in application or
  domain state. JaWS owns that live tree.

Definition dots should retain stable pointers to authoritative synchronized
state. Render and update through direct binders and synchronized getters over
that state.

Application code MUST NOT create a second representation of application state
for rendering. Page, screen, state, component, view-model, or DTO structs that
copy authoritative fields for templates, getters, attributes, or updates are
forbidden. Renaming such a copy does not make it acceptable. If a proposed type
exists only to carry render data copied from domain state, stop and redesign the
UI to read the authoritative state directly.

Application code MUST NOT use broad snapshots to manufacture fragment-wide
atomic rendering. JaWS requires race-free reads and correct dependency
invalidation, not a transaction across an entire fragment. Do not hold an
application lock across template execution to manufacture one. Separate reads
may observe adjacent valid states; dirty dispatch is the consistency mechanism
that makes matching Elements converge on current authoritative state.

When mutation code owns the writes, it MUST return or dirty the exact dependency
tags whose rendered output may change. It MUST NOT snapshot application state
only to diff it afterward and discover which ordinary mutation tags to dirty.
Derive that tag set directly from the mutation semantics.

Copying mutable collection storage under its lock so a caller can iterate after
unlock is a synchronization boundary, not a presentation snapshot. Capture
multiple primitives together only when one widget, attribute set, or operation
requires an invariant. Such a capture MUST remain local to that operation; it
must not become a retained render object, template Dot, or fragment-wide state
model.

For a Container, a freshly returned equal child is a reconciliation key. JaWS
reuses the existing Element and its original UI value; it does not replace
`elem.UI()` with the newly returned equal value. An unequal child removes the
old Element and renders a new one.

Dirtying only the parent reruns child selection. It does not update the output
of an unchanged retained child, so that child needs its own dependency tag.
Captured values that affect rendering must either participate in definition
equality or be read indirectly from synchronized mutable state.

## Choose the smallest rendering primitive

- When an authoritative source implements the getter and event interfaces for a
  standard widget, pass that source directly as the widget's primary argument,
  for example `{{$.Button .Action}}`. When the read is naturally a functor,
  adapt it with `bind.HTMLGetterFunc`; for text, use `bind.StringGetterFunc` and
  let the widget's `bind.MakeHTMLGetter` conversion escape it. A bound value can
  customize markup with `Binder.GetHTML` while remaining a standard
  `HTMLGetter`.
- An application UI may overload a standard widget's render or update behavior,
  but this is discouraged when a standard getter, binder, semantic `ui.Object`,
  or render parameter expresses the same control. Keep the overload only when it
  adds behavior the standard composition cannot provide, and document that
  behavior.
- HTML-inner getters run during initial rendering and dirty updates. If a getter
  queues wrapper attributes, `TailHTML` may repeat attributes already emitted
  inline. Treat an overload that suppresses that payload as a performance change:
  retain a benchmark and weigh the measured result against the simpler standard
  composition.
- **Full HTML document:** use `ui.Handler`. It creates the JaWS Request, applies
  `no-store`, renders without a generated wrapper, and treats the page Dot as
  arbitrary template data rather than a tag or equality key. The page
  Element is render-only; nested widgets own live updates. Do not call
  `NewRequest` before delegating to it and do not emulate a page with a bare
  `ui.Template`.
- **Static included fragment:** use Go's native template action. Pass `.Dot`
  when the fragment expects application data (`{{template "name" .Dot}}`), or
  `.` only when it intentionally expects the JaWS `ui.With` wrapper. Small
  static markup may simply remain inline.
- **Fixed live region:** use a wrapped `ui.Template` / `$.Template` partial.
- **Changing direct-child set, order, definition identity, or captured
  dependencies:** use Container, Tbody, or Select.

One `ui.Handler` reuses its configured Dot across HTTP requests, so that Dot
must support concurrent execution. For request- or session-specific data, use
an outer HTTP handler to load the data and invoke a newly constructed
`ui.Handler(jw, name, dot)` for that request. Put `SessionMiddleware` outside
the page handler when initial rendering needs a Session; `AutoSession` runs at
WebSocket upgrade and is too late for initial page state.

`ui.Handler` owns `NewRequest` and recognizes `jaws.ConnectHandler` in its
top-level Dot's method set, including promoted methods. It installs `JawsConnect`
before page template execution; the plain GET does not invoke it. An
implementation available only on a nested Template Dot is ignored without a
diagnostic. The bundled client connects after parsing the document. A custom
client can invoke the hook during rendering once flushed response bytes expose
the request key. Other Request setup requires a custom page handler. A
full-document Template is not a supported workaround. A connection identifies a
JaWS-capable client, not affirmative human intent; use a semantic click action
when that distinction matters.

A retained Template update keeps its wrapper Element and Jid, sends new inner
HTML, and unregisters/recreates managed descendants. It does not preserve
descendant identity, focus, scroll position, or client widget state.

## Definition identity and dependency tags

- Every non-nil UI used for reconciliation must be comparable at runtime and
  equal to itself. Keep slices, maps, functions, and NaN-bearing values out of
  definitions.
- Use Template, Container, Tbody, and Select definitions as values, not pointer
  wrappers. Their Dot/provider/handler participates in equality.
- A Template Dot is also an implicit tag. A comparable context struct registers
  that struct as one key; JaWS does not recursively register its fields.
- Make Dot dependencies deliberate. Implement a stable `JawsGetTag`, or return
  nil when children and explicit params own all dependencies. This changes tag
  expansion, never definition comparability or equality.
- A `JawsGetTag` result must not change with a mutable association. Register
  stable dependency identities on the provider. When that association selects
  different data, make the selected object's identity part of a reconstructed
  child definition and dirty the provider to reconcile it.
- Prefer pointers to authoritative data or fields as tags. Register shared group
  tags separately; do not return a group tag from an item's own `JawsGetTag`, or
  `Dirty(item)` expands into a group refresh.
- Dirty an Element for one request-local control. Dirty ordinary dependency tags
  for every matching Element across live Requests. Do not broaden dirty scope to
  hide missing dependencies.

## Bindings and semantic widget objects

Prefer `bind.New(&mu, &field)` for editable scalar state. It uses the real lock,
stores directly in the real field, and keeps `&field` as its stable tag through
chained hooks.

`SetLocked` replaces the previous setter. Validate or normalize while already
locked, then delegate the accepted write:

```go
binder := bind.New(&mu, &current).SetLocked(
	func(prev bind.Binder[int], elem *jaws.Element, next int) error {
		if next < minimum {
			next = minimum
		}
		return prev.JawsSetLocked(elem, next)
	},
)
```

Delegation preserves storage, `jaws.ErrValueUnchanged`, input reconciliation,
and whether `Success` runs. Do not assign the field directly, call public
`JawsSet`/`JawsGet`, or reacquire the same lock from a locked hook.

When a composed `ui.Object` or Binder owns content or initial attributes, pass
it as the widget's first getter argument:

```gotemplate
{{/* Content, click handler, tag, and initial attrs are all used. */}}
{{$.Button .Action `class="btn"`}}

{{/* Handler/tag may register, but Object content and initial attrs are ignored. */}}
{{$.Button "Run" .Action `class="btn"`}}
```

Do not pass the same Object in both positions; that duplicates handler/tag
registration. Construct fresh semantic Objects near the state and behavior they
represent rather than splitting one control into label, click, and attribute
helpers.

## Wrapper and initial-attribute lifecycle

- `$.Template` owns its generated wrapper and Jid. Its partial must not emit
  `id="{{$.Jid}}"` or forward wrapper attributes onto another root.
- HTML-inner widgets read initial attrs from their primary getter; input widgets
  read them from their primary binding/source; Template reads constructor attrs
  and attrs from Dot for its generated wrapper.
- Render params contribute literal attributes and register recognized handlers
  and tags. A parameter-valued `InitialHTMLAttrHandler` is not invoked.
- `ui.NewTemplate(tag, name, dot, attrs...)` accepts trusted raw wrapper attributes,
  which participate in Template equality. For duplicate names, precedence is render
  params, constructor attrs, then Dot attrs.
- Initial attrs run once for that Element. Dirty updates do not rerun them.
  Change dynamic attrs through Element update methods or replace the Element.
- A retained Template wrapper keeps its attributes while its recreated
  descendants run their own initial attrs.
- Object attribute hooks concatenate. Binder attribute hooks run with the Binder
  lock held and replace the earlier path unless they delegate to the previous
  Binder; do not re-enter the public Binder API.
- Keep widget-specific attributes out of a Binder reused by different widget
  kinds. For example, pass input-only `disabled` separately when the same Binder
  also renders a Span.

Plain strings passed to HTML-producing JaWS helpers are trusted raw HTML. Route
untrusted content through escaping Getter/Stringer forms. Build attributes from
untrusted values with `htmlio.Attr` and a trusted name; convert the result to
`string` for `NewTemplate`.

## Render shape and verification

- Before implementing, identify the authoritative state, its synchronization,
  the direct binders/getters that read it, and the precise dependencies whose
  rendered output each mutation can change. If the plan includes a
  render-specific copy of domain fields or a snapshot/diff layer for ordinary
  dirty tracking, stop and redesign it first.
- Keep HTML structure in templates and state mutations out of getter/render
  paths. When one direct getter result must remain consistent within a template
  execution, assign that value to a local template variable. This is not a
  reason to materialize the fragment as a render DTO or broad snapshot.
- Use direct field-backed binders and simple helpers before introducing custom
  binder, tag, or component abstractions.
- Test behavior with real JaWS Requests and Elements.
- Decode wire messages when identity matters: an equal retained fragment should
  keep its Jid and receive `Inner`; an identity change should produce the
  expected Remove/Append/Order operations.
- Use at least two live Requests or clients for cross-request dirtying, session,
  connect, and disconnect behavior.
- Test pure domain transitions separately from JaWS transport.

The following are completion blockers. Do not finish while the design contains
an application-owned UI tree, a full-document Template, any render DTO or
retained presentation snapshot, screen-shaped render state, pointer-wrapped
definition values, mutable tag identity, duplicate JaWS IDs, direct locked-field
assignment, snapshot/diff dirty tracking for mutation-owned writes, or broad
dirtying that masks dependency errors. Refactor the violation before continuing.
