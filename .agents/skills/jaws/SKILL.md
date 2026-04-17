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
- Prefer direct pointer tags to underlying data (for example `*Item`, `*Node`, `&state.field`) when identity is stable.
- Use getter-based values (`bind.StringGetterFunc` / `bind.HTMLGetterFunc`) for UI text/HTML that must reflect changing server state.
- Dirty only affected dependency tags after mutations, and include any derived-field dependencies that changed.
- Avoid synthetic tags (coordinates, ad-hoc strings, wrappers) when a stable underlying data pointer exists.

## Hard framework constraints

- Every JaWS `UI` value must be comparable.
- `Container.JawsContains` must return hashable `UI` items, and the returned slice must not be mutated after return.
- Treat `*ui.Container` / `*ui.Tbody` / `ContainerHelper` widgets as render-scoped; construct fresh per render, do not cache across requests.

## Template-dot and tag rules

- `ui.Template` expands `Dot` into tags via `jtag.TagExpand`; the root dot is part of identity/tag behavior.
- Prefer comparable root dots (pointers or small comparable structs).
- If root dot is non-comparable, implement `JawsGetTag(jtag.Context) any` and return a comparable tag.
- Do not use plain `string`, numeric, `bool`, `template.HTML`, or `template.HTMLAttr` as tags; these are illegal tag types.
- If you need string-like semantic tags, use `jtag.Tag("...")` or a comparable typed struct/pointer.

## `$.Template(...)` parameter semantics

JaWS parses template params as:
- HTML attrs: `string`, `[]string`, `template.HTMLAttr`, `[]template.HTMLAttr`
- handlers: `EventFn`, `EventHandler`, `ClickHandler`
- tags: everything else (plus comparable handlers are auto-tagged)

Implications:
- Non-comparable handlers are not auto-tagged unless they implement `TagGetter`.
- Pass explicit tags when dirty targeting depends on them.
- Include wrapper markup attributes via `{{$.Attrs}}`.
- For dynamic button text, avoid passing plain static strings if the value must change after render; use getter-based values so updates reflect new state.

## Event handling model

On incoming events, JaWS dispatches in this order:
1. UI object (`elem.Ui()`), `JawsClick` first for click events, then `JawsEvent`
2. Additional handlers attached to the element, in registration order

Use `jaws.ErrEventUnhandled` to intentionally fall through to the next handler.

## Clickable template pattern

For clickable content rendering:
- Prefer a template dot with `JawsClick` over passing redundant explicit click handlers.
- Use explicit click handler params only when dot-owned handling is not viable.
- Wrapper template root should include `id="{{$.Jid}}" {{$.Attrs}}`.
- Add interaction semantics where needed, for example `role="button" tabindex="0"`.
- Keep body partials presentational; attach behavior at wrapper/dot level.

## Rendering and update rules

- Keep HTML structure in templates; avoid manual HTML string assembly in Go.
- When using HTML getters, keep getter paths pure reads.
- Use `JawsUpdate` for incremental updates when a custom widget needs it.
- `Element.SetAttr/RemoveAttr/SetClass/RemoveClass/SetInner/SetValue/Append/Order/Remove/Replace` are update-time operations; call them only from render/update processing.
- Remember that widgets such as `ui.Button` update inner HTML from the original getter object; if that getter captured a stale static value, dirtying will not refresh the UI.

## Dirtying rules

- Prefer `Request.Dirty(...)` when in request context.
- Avoid `Jaws.Dirty(...)` unless necessary; its tag expansion runs with nil request context.
- Dirty only precise tags whose output depends on the changed state.
- Avoid broad model-level dirty tags when finer-grained element-level tags are practical.
- For broad refreshes, attach a shared dependency tag to all relevant elements and dirty that shared tag instead of enumerating many element tags.

## HTML safety rules

`bind.MakeHTMLGetter` behavior is type-dependent:
- `string` is used as raw HTML and is not escaped.
- `template.HTML` is trusted as-is.
- `Getter[string]`, `Binder[string]`, `fmt.Stringer`, and formatter-based paths are escaped.

Guideline:
- Never pass untrusted input as plain `string` to HTML-producing helpers.

## Request/session integration rules

- Ensure pages include both `HeadHTML` and `TailHTML` in layout flow.
- `TailHTML` helps apply queued attr/class updates immediately and reduce initial flicker.
- Register JaWS `/jaws/*` routes correctly and pair request creation with `UseRequest` handling.
- Session storage is server-side and IP-bound; treat `Request.Get/Set` as session-backed convenience helpers.

## Runtime/lifecycle cautions

- Start JaWS processing loop (`Serve`/`ServeWithTimeout`) before relying on broadcast-driven APIs.
- `Broadcast`-driven helpers (including session reload/close flows) may block before the serve loop is running.

## Testing checklist

- Use real JaWS requests/elements for render/click/update tests.
- Add regression tests for click dispatch when moving handlers between params and dot `JawsClick`.
- For container regressions, verify identity reuse, append/remove/order behavior, and stale-element cleanup.
- If rerendering fails, inspect tag comparability and dirty-target coverage before broadening dirty scope.

## Anti-patterns

- Repo-specific abstractions that hide JaWS contracts instead of modeling them.
- Fake binders or fake tags created only to satisfy an API shape.
- Hidden mutations in getter paths.
- Broad `Dirty(...)` calls used to mask incorrect dependency targeting.
- Passing explicit template click handlers when dot-owned `JawsClick` already covers behavior.
