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

## Dirtying rules

- Prefer `Request.Dirty(...)` when in request context.
- Avoid `Jaws.Dirty(...)` unless necessary; its tag expansion runs with nil request context.
- Dirty only precise tags whose output depends on the changed state.

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
