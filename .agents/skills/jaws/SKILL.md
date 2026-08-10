---
name: jaws
description: Use this skill when implementing or refactoring server-driven UI with github.com/linkdata/jaws, including templates, handlers, dirtying, tags, sessions, and render/update behavior.
metadata:
  short-description: Build apps with JaWS
---

## When to apply this skill

Apply these rules whenever work involves any of the following:
- Go code that creates JaWS `UI` values, binders, handlers, requests, or sessions
- Go templates rendered through `ui.Template` / `$.Template`
- Event handling, dirtying, tag identity, or dynamic container updates

## Primary objective

Keep browser behavior thin and deterministic while preserving server-side truth, stable identity, and predictable rerenders.

## Framework mindset

JaWS is an immediate-mode, server-driven UI framework, not an MVC framework.
- Treat render output as a direct projection of current server state.
- Keep authoritative state in domain data, not duplicated in UI-specific state layers.
- Use tags to express data dependencies so rerenders are targeted and deterministic.

## Practical data/tag alignment

- Model interactive units as first-class objects and keep related behavior on those objects where practical.
- Prefer direct pointer tags to underlying data (for example `*Item`, `*Node`, `&state.field`) when identity is stable. `bind.New(&mu, &field)` follows this pattern automatically: its tag is always `&field`, unchanged by any chained hooks.
- Use getter-based values (`bind.StringGetterFunc` / `bind.HTMLGetterFunc`) for UI text/HTML that must reflect changing server state.
- Dirty only affected dependency tags after mutations, and include any derived-field dependencies that changed.
- Avoid synthetic tags (coordinates, ad-hoc strings, wrappers) when a stable underlying data pointer exists.
- For collection elements, register each element with both its item-level tag (the item's pointer) and a shared group tag (for example `&g.items`). Mutations can then dirty a single item, several items, or the whole group from the same tag namespace, without changing what each element listens to.
- Keep an item's own `JawsGetTag` scoped to the item, never returning the shared group tag. Element *registration* (what an element listens to) and dirty-target *expansion* (what `Request.Dirty(x)` resolves to) are different things. If an item type implements `tag.TagGetter` and its `JawsGetTag` returns `[]any{item, &group}`, then `Request.Dirty(item)` tag-expands to include `&group` and re-renders **every** element registered under it — silently defeating per-item dirtying even though you passed a single item. Return only the item's own identity from `JawsGetTag`, and attach the shared group tag separately at construction (for example pass `&group` as an extra tag param to the item's widget). The element then listens to both, but `Dirty(item)` stays scoped to that one item while `Dirty(&group)` still refreshes the whole group.

## Hard framework constraints

- Every JaWS `UI` value must be a non-nil interface, comparable at runtime, **and
  equal to itself**, since it is used as a map key. A value that is a nil interface,
  only statically comparable (an interface field holding a slice/map/func), or that
  holds a `NaN` (so `v != v`) is rejected: the container widgets — the only place a raw
  `UI` value is used as a map key — cancel the `Request` in all builds (cause matches
  `tag.ErrNotUsableAsTag`), and `Request.NewElement` asserts runtime comparability in
  debug builds. A typed nil (e.g. `(*Widget)(nil)`) is comparable and equal to itself,
  so it counts as usable and JaWS dispatches render/update/event calls to it; only a
  nil `UI` interface is a no-op. Surviving such a call is up to the concrete type, not
  a requirement: a widget that dereferences its fields panics, and none of the
  standard `lib/ui` widgets document nil-receiver tolerance. Do not pass a nil pointer
  of a type that does not; use a zero value only where that type documents one.
- Every JaWS `UI` value is request-scoped. Once used by one Request, never use
  that value with another Request; construct fresh widgets per request. The
  widgets may still refer to shared, synchronized application state, binders,
  handlers, and tags.
- Within its Request, a `UI` value normally backs one live `Element`. Reuse one
  value for multiple live Elements only when its concrete type documents that
  support and either retains no Element-specific state or keeps it separately on
  each Element. `ui.Container`, `ui.Tbody`, `ui.Select` and `ui.Template` use the
  per-Element state-slot form.
- `jaws.Container.JawsContains` must return `UI` items that are comparable and equal to
  themselves (see above); returning one that is not cancels the `Request`. The
  returned slice must not be mutated after return. A UI value may occur more than
  once in one returned slice only when its type supports multiple live Elements. Each
  child must render one addressable direct DOM node carrying its Element's JaWS ID so
  removal and ordering can target it; `ui.NewTemplate` provides that node through its
  generated wrapper.
- Treat the package documentation shown by
  `go doc github.com/linkdata/jaws/lib/ui` as the canonical standard-widget
  multiplicity summary, and consult each concrete type's docs for its conditions.

## Constructing UI values: `ui.New` and `bind.New`

These are the two usual building blocks for widget handlers passed to `$.Button`, `$.Span`, `$.A`, `$.Div`, `$.Label`, `$.Text`, `$.Range`, etc. Pick based on what the widget represents: presentational content (`ui.New`) versus editable state backed by a variable (`bind.New`).

### `ui.New(innerHTML any) Object`

- `innerHTML` is routed through `bind.MakeHTMLGetter`, whose first matching
  conversion applies: `HTMLGetter` is unchanged; `template.HTML` is trusted;
  `Binder[string]`, `Getter[string]`, and `fmt.Stringer` adapters are escaped;
  `string` is trusted; other values are formatted and escaped.
- The adapters created for `Getter[string]` and `fmt.Stringer` inputs expose the
  wrapped value as an implicit tag. It must be a usable direct tag or implement
  `tag.TagGetter` and return values accepted by `tag.TagExpand`; return `nil` from
  `JawsGetTag` when the value should be untagged.
- Returns a chainable `Object`. Each builder — `.Clicked(fn)`, `.ContextMenu(fn)`, `.InitialHTMLAttr(fn)` — wraps the previous object so the chain is a linked list, newest first.
- Event dispatch walks the chain newest-first and stops at the first link that does not return `jaws.ErrEventUnhandled`. Return `ErrEventUnhandled` from a link to fall through to the next.
- `JawsInitialHTMLAttr` concatenates attribute strings from every link that defines one.
- `JawsGetTag` collects non-nil tags from every link; this is the tag set used by `tag.TagExpand` for dirty targeting.
- Use `ui.New` for buttons, spans, labels, and similar content-plus-behavior widgets where the identity of the underlying state is not the point.

### `bind.New[T comparable](l sync.Locker, p *T) Binder[T]`

- Signature: `func New[T comparable](l sync.Locker, p *T) Binder[T]`. If `l` also satisfies `RWLocker` (has `RLock`/`RUnlock`), the binder takes the read lock for reads; otherwise it upgrades to the write lock.
- `T` must be strictly comparable. Interface types, including `any`, are unsupported; an array's element type and every struct field type must also be strictly comparable, regardless of the bound values. The default setter comparison may panic when `T` satisfies the `comparable` constraint but is not strictly comparable.
- The binder's tag is always `p` (the pointer itself). Chaining never changes tag identity — `bind.New(&mu, &field).Clicked(...).Success(...)` still reports `&field` as its tag, so dirty targeting via `&field` keeps working through refactors.
- Default `JawsSetLocked` assigns `*p = v` only when the value changed and returns `jaws.ErrValueUnchanged` when it did not. This is what lets the input-widget family skip redundant updates.
- Chain builders return a new `Binder[T]`:
  - `.SetLocked(fn)` / `.GetLocked(fn)` — override read/write semantics.
  - `.GetHTML(fn)` — supply a custom `JawsGetHTML` for HTML-rendering widgets (`Span`, `Div`, `A`, `Label`).
  - `.Success(fn)` — run after a successful set. Accepted signatures: `func()`, `func() error`, `func(*Element)`, `func(*Element) error`. Non-nil errors propagate; a handler can still return `ErrValueUnchanged` to signal no change.
  - `.Clicked(fn)` / `.ContextMenu(fn)` — attach click/context handlers to the same bound variable.
  - `.InitialHTMLAttr(fn)` — attach attribute hooks.
- Use `bind.New` for input widgets and for content whose natural key is the backing variable. Multiple widgets bound to the same pointer share a tag automatically, so `Request.Dirty(&field)` refreshes all of them.
- Writable setters used by `ui.Input`-based widgets require a stable target
  accepted by `Element.ApplyGetter` for post-set reconciliation. `bind.New` supplies
  its backing value pointer; otherwise use a pointer-valued setter or `JawsGetTag`
  returning at least one stable, usable key. A `JawsGetTag` result takes precedence
  over the setter's identity.
- Render-param tags register the Element but do not replace that setter-derived
  target. Editable Number and Range sources fail rendering without a valid target.
  Other Input-based widgets render, but cannot automatically reconcile rejected or
  normalized browser values without one.

### When to use which

- `ui.New` when the widget is presentational or event-driven and identity flows from the containing object (for example a cell that already has a `*Cell` tag registered as a dep).
- `bind.New` when the widget reads or writes a specific variable and you want the variable's address to be the stable dirty target.
- Either form accepts additional tag arguments where the API allows it (`bind.HTMLGetterFunc(fn, deps...)`, `bind.StringGetterFunc(fn, deps...)`) so a single widget can depend on several pieces of state without wrapping types.

### HTML-inner constructors

- `ui.NewDiv`, `ui.NewSpan`, `ui.NewButton`, `ui.NewA`, `ui.NewLabel`, `ui.NewTd`, `ui.NewTr`, and `ui.NewLi` accept `innerHTML any` and call `bind.MakeHTMLGetter` internally.
- The matching `RequestWriter` helpers (`$.Div`, `$.Span`, etc.) have the same content conversion rules because they call the constructors.
- Plain strings are trusted raw HTML at both levels. Use `bind.Getter[string]`, `bind.StringGetterFunc`, `fmt.Stringer`, or explicit escaping for untrusted text.

## Container-family constructors and identity

- `ui.NewContainer(outerHTMLTag, children)`, `ui.NewTbody(children)` and
  `ui.NewSelect(handler)` return definition **values**, not pointers. Treat them as
  immutable after use and do not take their addresses; pointers replace definition
  equality with pointer identity. Tbody embeds a Container configured for `tbody`;
  replacing it is unsupported.
- The provider or handler is part of widget equality. Its dynamic value must be
  comparable at runtime and equal to itself. A slice, map, function, interface holding
  one of those, or a NaN-bearing value makes the whole widget unusable as a container
  child even though its Go struct is statically comparable.
- Keep slice/map/function-bearing application objects behind stable pointers and pass
  the application-object pointer to the constructor. Rebuild with that **same pointer**
  to obtain an equal widget and Element reuse. Mutate only synchronized application
  state behind the pointer; never mutate the widget definition or allocate a fresh
  pointer merely to rebuild it.
- Equal values may back multiple live Elements within one Request when their providers
  or handlers are safe for all calls and each child UI value reused across those Elements
  supports multiple live Elements. They remain request-scoped; construct fresh widget
  values for another Request even when those values refer to shared synchronized
  application state.
- A nil-interface child provider is a valid part of the Go value but is not renderable:
  zero Container/Tbody and `NewContainer`/`NewTbody` with nil providers panic when
  render/update calls `JawsContains`. Zero Select and `NewSelect(nil)` likewise panic in
  render/update, while `Select.JawsInput` treats the nil handler as a no-op. A typed-nil
  provider is dispatched normally and must tolerate its nil receiver itself.

## Template-dot and tag rules

- `ui.Template` expands `Dot` into tags via `tag.TagExpand` (package `github.com/linkdata/jaws/lib/tag`, imported as `tag`); the root dot is part of identity/tag behavior.
- `ui.Template` is for partial templates only; full document/page templates should be rendered through `ui.Handler`.
- A nil-interface Template `Dot` is valid and contributes no tag; a typed nil follows its
  dynamic type's comparability and expansion behavior.
- The root dot **must** be comparable at runtime and equal to itself: `ui.NewTemplate`
  returns a value, so the dot is part of the widget the container widgets use as a map key.
  A slice, map, func or NaN-bearing dot makes the widget unusable as a container child.
- Implementing `JawsGetTag() any` does **not** fix a non-comparable dot — it
  resolves the *tag*, not the widget's comparability. A non-comparable dot is unsupported.
  Always use the Template itself as a value; taking its address is unsupported because it
  changes container reuse to pointer identity. `ui.Handler` is the arbitrary-dot exception.
- `JawsGetTag` is the canonical public accessor for an object's tags, and application code may
  call it directly. Callers needing flattened, validated keys pass the object to
  `tag.TagExpand`. It takes no context argument: a tag value expands the same way regardless
  of which request or goroutine expands it.
- `Element.ApplyGetter` invokes `JawsGetTag` to obtain a tag candidate, then expands that
  candidate for registration. This expansion may invoke `JawsGetTag` again when the candidate
  is itself a `tag.TagGetter` or contains one. Standard getter-backed widgets invoke
  `ApplyGetter` once during initial render, but there is no `JawsGetTag` call-count guarantee:
  `tag.TagExpand`, dirtying, broadcasts, and application code may make further calls.
- Except for an explicitly documented initialization phase that returns nil, a `tag.TagGetter`
  must be idempotent in tag identity. After its first non-nil result, every call must return a
  value that `tag.TagExpand` expands to the same set of keys. Previously returned containers
  must continue expanding to the key set they produced when returned and must be treated as
  read-only. Fresh containers and equivalent representations are allowed. Non-idempotent
  `tag.TagGetter` implementations are unsupported.
- JaWS does not serialize `JawsGetTag` calls. A getter used concurrently must synchronize its
  state and safely publish any returned containers.
- `ui.JsVar` is the one in-tree initialization case: `JawsGetTag` returns nil before its first
  render initializes the dirty tag, and that nil is not a dirty target. A getter in its nil phase
  is therefore not usable as a tag: passing a not-yet-rendered `JsVar` as a tag to another widget
  expands to no keys, so that widget registers under nothing and a later dirty of the JsVar never
  reaches it. Render the JsVar first, or tag the other widget with an independent value. `ui.Object`
  propagates the phase of any chained getter that has one.
- `tag.TagExpand` rejects exactly these as tags: `string`, `bool`, `int`/`int8`/`int16`/`int32`/`int64`,
  `uint`/`uint8`/`uint16`/`uint32`/`uint64`, `float32`/`float64`, `template.HTML`, `template.HTMLAttr`,
  `jid.Jid` and `key.Key`. It is a switch on exact types, so aliases of a rejected type are rejected,
  while `uintptr` and the complex types are not on the rejection list. Other defined types
  (`type RowID string`) are not rejected merely because their underlying predeclared type is on it —
  they still have to be comparable and equal to themselves, and one implementing `tag.TagGetter` is
  expanded instead.
- This applies to a Template's `Dot` too, since rendering expands it: a comparable, reflexive `string`
  dot still fails at render with `illegal tag type string`.
- If you need string-like semantic tags, use `tag.Tag("...")` or a comparable typed struct/pointer.

## `$.Template(...)` signature and parameter semantics

`$.Template(...)` takes the wrapper tag first:

```gotemplate
{{$.Template "div" "partialName" .Dot "class=\"panel\""}}
{{$.Template "tr" "rowPartial" . "class=\"selected\""}}
```

The outer tag should match the DOM context where the generated JaWS wrapper will
be inserted. An empty outer tag selects the default `div` wrapper. Use a semantic
wrapper such as `tr`, `td`, `li`, or `option` where the DOM context requires it.

For a static structural fragment that needs no JaWS-managed wrapper, use Go's native
template inclusion so the surrounding Template owns the DOM:

```gotemplate
{{template "barePartial" .Dot}}
```

JaWS parses template params as:
- HTML attrs: `string`, `[]string`, `template.HTMLAttr`, `[]template.HTMLAttr`
- Handlers: `InputFn` (the func alias `func(e *Element, val string) error`), plus anything satisfying `InputHandler`, `ClickHandler`, or `ContextMenuHandler`
- Tags: everything else (plus comparable handlers are auto-tagged)

Implications:
- Non-comparable handlers are not auto-tagged unless they implement `tag.TagGetter`.
- Pass explicit tags when dirty targeting depends on them.
- HTML attributes passed to `$.Template(...)` are applied to the generated template wrapper.
- Template bodies used with `$.Template(...)` must be partials, not full documents.
- For dynamic button text, avoid passing plain static strings if the value must change after render; use getter-based values so updates reflect new state.

## Registering template-authored elements

- `$.Register(updater, params...)` binds a render-independent `jaws.Updater` to a DOM
  element whose markup is written by the surrounding template. The returned Jid must be
  used as that element's HTML `id`:

  ```gotemplate
  <section id="{{$.Register .Dot.Panel}}">...</section>
  ```

- Register never calls `JawsRender`; use it only for a custom updater designed to work
  without render-time initialization. It uses the updater as a tag, attaches supported
  click and context-menu handlers, applies tag and supported handler params, and invokes
  `JawsUpdate` once. HTML attribute params are ignored; write attributes on the
  template-authored element.
- The updater must be a non-nil interface whose dynamic value is comparable at runtime,
  equal to itself, and usable as a tag. A typed nil is invoked normally and must tolerate
  its nil receiver. Reuse one updater for live Elements only when it supports that use
  without retaining Element-specific state on the shared value; it must be safe for
  concurrent use when shared across Requests.
- Prefer ordinary widget rendering. The container family supports update-only
  registration with the limitations below, and `ui.Select` is its input-handling
  exception. Template-authored `input`, `textarea`, and `contenteditable` elements,
  the typed input widgets, `ui.Number`, `ui.Range`, `ui.JsVar`, and Templates with
  a non-empty `OuterHTMLTag` require rendering.
- Always emit the returned Jid as the element's `id`. A surrounding Template owns the
  registered Element; otherwise it remains until explicit deletion, reported DOM
  removal, or Request shutdown.

## Event handling model

On incoming events, JaWS dispatches in this order:
1. Handlers attached to the element are tried in **reverse registration order** (most recently added first).
2. If every attached handler returned `ErrEventUnhandled` (or none matched the event kind), the UI object itself (`elem.UI()`) is called last as the fallback.

The handler candidate is asked via `JawsClick` / `JawsContextMenu` / `JawsInput`, matched to the event kind; there is no generic `JawsEvent` method. Return `jaws.ErrEventUnhandled` to fall through to the next candidate.

## Clickable template pattern

For clickable content rendering:
- Prefer a template dot with `JawsClick` over passing redundant explicit click handlers.
- Use explicit click handler params only when dot-owned handling is not viable.
- `ui.Template` creates the outer JaWS wrapper; template roots should not declare the JaWS ID or carry forwarded wrapper attributes.
- Add interaction semantics where needed through Template params, for example `role="button"` and `tabindex="0"`.
- Keep body partials presentational; attach behavior at wrapper/dot level.

## Rendering and update rules

- Keep HTML structure in templates; avoid manual HTML string assembly in Go.
- `Element.ApplyGetter` applies a primary getter's tag and event-handler interfaces;
  it does not invoke `InitialHTMLAttrHandler`. Collect initial attributes separately
  with `Element.ApplyInitialHTMLAttr`, without holding widget or bound-value locks that
  the callback may acquire. The handler owns its synchronization. A `bind.Binder`
  acquires its own lock before invoking its `InitialHTMLAttrHook`.
- `ui.Template.JawsUpdate` re-renders the template data into the generated wrapper.
- `ui.Container`, `ui.Tbody` and `ui.Select` claim a private `containerState` on each
  Element before registering tags or handlers, invoking application code, or writing
  output. Contention returns `jaws.ErrElementStateClaimed` with no render side effects.
  The state holds the render-time dirty tag, reconciliation mutex and owned child
  Elements; the widget value retains only its definition.
- Container-family updates normally load that state. Update-only use through
  `$.Register` lazily claims a missing state before invoking the provider. A foreign
  state, typed-nil state or lost concurrent claim is reported through `MustLog`, and no
  children are reconciled; Select also sends no value. The lazy state has no render-time
  dirty tag. `$.Register` delivers Select input to its handler but does not call
  `Element.ApplyGetter` for that handler, so Select retains no handler-derived tag for
  its own post-set dirtying. Any handler-initiated dirtying still occurs, and separately
  registered tags remain registered. Use ordinary `$.Select` rendering when Select
  should register and use a usable tag exposed by its handler.
- Container ownership also lives in `containerState`, not in `elem.UI()`. Cleanup
  detaches children under the state mutex and recurses after unlocking, so it also finds
  children when the Element's visible UI is the private registration wrapper. Failed
  render and append paths unregister every child and nested owner they created.
- Provider callbacks and child validation run without the state mutex. Reconciliation
  holds it only while matching children and calling `Request.NewElement`; rendering,
  removal, cancellation, recursive cleanup and logging happen after it is released.
- On a successful render, Select queues its selected value after rendering its options.
  After a non-contended reconciliation returns, it queues the value; state contention
  suppresses both operations.
- Container reconciliation is parent-local and updates direct children only. Reordering
  retained equal direct children preserves their Elements and complete nested subtrees.
  Each nested Container, Tbody or Select whose own children changed needs its own
  dirty/update pass; updating an outer parent is not recursive. Child Element identity
  is scoped to its parent, so moving a child definition between parents does not preserve
  its Element.
- `ui.NewTemplate` returns a plain `ui.Template` **value** that may back multiple live
  Elements because it keeps the Elements its execution creates in each rendering Element's
  state slot rather than on itself. Do not take its address. A
  successful update unregisters every Element the previous execution created through the
  writer it was given — the widget helpers, `$.Register`, `$.RadioGroup` and nested
  `$.Template` alike — since `SetInner` replaces the DOM holding them. Ownership is recorded
  at creation, so an Element that never rendered is reclaimed as well.
- Because the value is stateless, a container's `JawsContains` may rebuild equal children on
  every call and their Elements are still reused — that equality *is* the reuse key.
- The state slot has one claimant, which constrains composition:
  - at most one state-owning Template or container-family renderer may render a given
    Element;
  - a composite UI must use Template values equal under `==` for rendering and updating
    an Element; using unequal values is unsupported;
  - a Template with a non-empty `OuterHTMLTag`, including any returned by
    `ui.NewTemplate`, updates only an Element rendered by an equal Template value, so it
    is not usable as a `$.Register` updater; `$.Register` never invokes its renderer and
    therefore cannot establish the wrapper state an update needs.
- Call `$.RadioGroup` from the template that renders the group: its Elements belong to
  the template whose body called it, not to the wrapper their markup lands in.
- HTML getter paths must not mutate domain state, but they may call element update methods (`SetClass`, `RemoveClass`, `SetAttr`, `RemoveAttr`, etc.) on the passed-in `*Element` to co-ordinate wrapper class/attribute changes with the inner-HTML refresh. No custom `JawsUpdate` is needed for that case — the queued wrapper updates flush alongside the `SetInner` from `HTMLInner.JawsUpdate`.
- Use a custom `JawsUpdate` only when the widget's update logic diverges from "render the getter again" — e.g. to compare against a stored last-value and skip the update (as the input widgets do).
- `Element.SetAttr/RemoveAttr/SetClass/RemoveClass/SetInner/SetValue/Append/Order/Remove/Replace` are update-time operations; call them only from render/update processing.
- Pass the Element itself to `Dirty` when an event must reconcile only that browser-local control. Dirty dependency tags when application state changes so every dependent Element is refreshed.
- Remember that widgets such as `ui.Button` update inner HTML from the original getter object; if that getter captured a stale static value, dirtying will not refresh the UI.

## Dirtying rules

- `Request.Dirty` and `Jaws.Dirty` are equivalent. Both expand through `Jaws.MustTagExpand`.
  An expanded non-nil `*Element` is treated as an exact target on its owning Request;
  other keys dirty every matching Element across all live Requests. `Request.Dirty` is
  not scoped to its own Request for ordinary tags.
- Both selector kinds use the normal batched dirty pass and require the JaWS serving
  loop to be running.
- Dirty only precise tags whose output depends on the changed state.
- Avoid broad model-level dirty tags when finer-grained element-level tags are practical.
- For broad refreshes, attach a shared dependency tag to all relevant elements and dirty that shared tag instead of enumerating many element tags.
- `Request.Dirty` runs the tag list through `Jaws.MustTagExpand`, which has a hard cap of 100 expanded entries. Above that, expansion fails with `tag.ErrTooManyTags`: with a `Jaws.Logger` configured the error is logged and the partial expansion is still applied, and without one the call panics before anything is dirtied. When a mutation might touch more items than that, prefer the shared group tag over enumerating individual item tags.
- `Request.TagsOf(elem)` reports every tag actually registered on an element, including tags added separately from the UI object — use it when a dirty target seems not to reach an element.
- Redundant-update filtering is asymmetric: input widgets (`InputText`, `InputBool`, `InputDate`, `Number`, `Range`) cache their browser or bound state and normally skip `SetValue` when the getter output is unchanged, but `HTMLInner`-backed widgets (spans, divs, buttons) do not — `JawsUpdate` unconditionally calls `SetInner`. The Number and Range handlers invalidate their text baseline when rejected input must be restored and reconcile accepted text with canonical formatting when needed. For HTML-inner widgets, ensure dirty scope matches fields that actually changed, otherwise unrelated status/label spans will re-render (and lose selection, transitions, etc.) on every event. Usually the mutation code already knows what it changed and can dirty accordingly; fall back to snapshot-and-diff only when outcomes are hard to predict up front (e.g. flood-fill or win-condition checks) and the snapshot is cheap.

## HTML safety rules

`bind.MakeHTMLGetter` uses the first matching conversion:
- `HTMLGetter` is returned unchanged.
- `template.HTML` is not escaped.
- `Binder[string]`, `Getter[string]`, and `fmt.Stringer` adapters are escaped.
- `string` is not escaped.
- Other values use escaped `fmt.Sprint` output.

Guideline:
- Never pass untrusted input as plain `string` to HTML-producing helpers.

## Request/session integration rules

- Ensure pages provide the configured JaWS resources and Request key metadata;
  `HeadHTML` is the usual way to emit them.
- `TailHTML` is optional; it applies queued attr/class updates before the
  WebSocket connects and can reduce initial flicker.
- Register the JaWS `/jaws/` route prefix correctly and pair request creation with `UseRequest` handling.
- Session storage is server-side and IP-bound; use `Jaws.SessionMiddleware(...)` when page state should be per-user.
- For per-session app state, load from `Request.Get(key)` and initialize with `Request.Set(key, value)` during the page request.

## Runtime/lifecycle cautions

- Start the JaWS processing loop (`Serve`/`ServeWithTimeout`) before relying on broadcasts or dirty updates.
- `Broadcast`-driven helpers (including session reload/close flows) may block before the serve loop is running.

## Testing checklist

- Use real JaWS requests/elements for render/click/update tests.
- Add regression tests for click dispatch when moving handlers between params and dot `JawsClick`.
- For container regressions, verify identity reuse, append/remove/order behavior, and stale-element cleanup.
- For container-family state changes, verify equal values keep independent Elements,
  contention precedes callbacks/output, update-only registration claims lazily, and
  nested ownership cleanup keeps the Request registry flat.
- Add pure domain tests for state transitions (win/loss, reset, bounds checks) independent of JaWS transport.
- If rerendering fails, inspect tag comparability and dirty-target coverage before broadening dirty scope.

## Anti-patterns

- Repo-specific abstractions that hide JaWS contracts instead of modeling them.
- Fake binders or fake tags created only to satisfy an API shape.
- Hidden mutations in getter paths.
- Broad `Dirty(...)` calls can mask incorrect dependency targeting.
- Pointer-wrapping a Container, Tbody, Select or Template value, which replaces
  definition equality with pointer identity.
- Passing a runtime-incomparable application object directly to a container-family
  constructor instead of retaining it behind a stable pointer.
- Using `$.Register` for a widget that can render its own element, or failing to place
  its returned Jid on the template-authored DOM node the updater controls.
- Returning a shared/group tag from an item's `JawsGetTag` (bundling it into the item's own dirty identity), which makes a single-item `Dirty` fan out to the whole group.
- Passing explicit template click handlers when dot-owned `JawsClick` already covers behavior.
- Adding custom browser JavaScript for state that can be expressed through JaWS events and server updates.
