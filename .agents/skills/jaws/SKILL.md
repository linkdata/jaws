---
name: jaws
description: Build or refactor Go UIs with github.com/linkdata/jaws. Use for page and partial rendering, containers, binders, events, dependency tags, sessions, or updates.
metadata:
  short-description: Build apps with JaWS
---

# JaWS application design

Build the smallest direct projection of synchronized server state. JaWS is an
immediate-mode, server-driven UI framework, not MVC.

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

For a Container, a freshly returned equal child is a reconciliation key. JaWS
reuses the existing Element and its original UI value; it does not replace
`elem.UI()` with the newly returned equal value. An unequal child removes the
old Element and renders a new one.

Dirtying only the parent reruns child selection. It does not update the output
of an unchanged retained child, so that child needs its own dependency tag.
Captured values that affect rendering must either participate in definition
equality or be read indirectly from synchronized mutable state.

## Choose the smallest rendering primitive

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

`ui.Handler` owns `NewRequest` and exposes no Request setup hook. If a design
depends on `SetConnectFn`, either move that lifecycle into supported HTTP/session
setup or consciously build a custom page handler. A full-document Template is
not a supported workaround.

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
  read them from their primary binding/source; Template reads them from Dot for
  its generated wrapper.
- Render params contribute literal attributes and register recognized handlers
  and tags. A parameter-valued `InitialHTMLAttrHandler` is not invoked.
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
untrusted text through escaping Getter/Stringer forms or escape it explicitly.

## Render shape and verification

- Keep HTML structure in templates and state mutations out of getter/render
  paths. Bind a repeated state read once per template execution when a fragment
  needs an internally consistent view.
- Use direct field-backed binders and simple helpers before introducing custom
  binder, tag, or component abstractions.
- Test behavior with real JaWS Requests and Elements.
- Decode wire messages when identity matters: an equal retained fragment should
  keep its Jid and receive `Inner`; an identity change should produce the
  expected Remove/Append/Order operations.
- Use at least two live Requests or clients for cross-request dirtying, session,
  connect, and disconnect behavior.
- Test pure domain transitions separately from JaWS transport.

Before finishing, confirm the design does not introduce an application-owned UI
tree, a full-document Template, pointer-wrapped definition values, mutable tag
identity, duplicate JaWS IDs, direct locked-field assignment, or broad dirtying
that masks dependency errors.
