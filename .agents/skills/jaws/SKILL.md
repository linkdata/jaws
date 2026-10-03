---
name: jaws
description: Design, implement, or review Go UIs using github.com/linkdata/jaws. Find version-matched documentation and validate JaWS design decisions about templates, widgets, bindings, attributes, tags, and updates.
metadata:
  short-description: JaWS guides and design checklists
---

# JaWS

## Find the matching documentation

In a JaWS checkout, use its `doc/README.md`. In a consuming application, find
its selected module source, including a local `replace`, with:

```sh
go list -m -f '{{.Dir}}' github.com/linkdata/jaws
```

Open `doc/README.md` there. All guide paths below are relative to that module
root. Use `go doc <package>.<Symbol>` for exported API contracts. The
[online wiki](https://github.com/linkdata/jaws/tree/main/doc) describes the
development branch and may differ from the selected version.

## Validate each design decision

**Every time you make or change a decision below, check its checklist against
the implementation.** Resolve failed checks before proceeding; run interaction
checks once that interaction is runnable. Read the named
local guide for details. Skip checklists for decisions not touched; do not add
review artifacts unless requested.

### Choose state ownership and bindings

Guide: `doc/bindings.md`, `doc/sessions.md`.

- [ ] Use application objects directly. Put bindings, computed content, actions,
  attributes, and tag methods on them; avoid copied presentation state.
- [ ] Distinguish shared, session, and Request-local state. Synchronize mutable
  state used by concurrent callbacks, including within one Request. Keep
  published identity fields immutable; whole-struct assignments write them too.
- [ ] Give editable controls writable sources with stable tags. Keep validation
  and authorization in setters or actions, not just browser attributes.

### Choose page structure

Guide: `doc/ui/README.md`, `doc/ui/controls.md`.

- [ ] Use standard Go `define`, `template`, `block`, `if`, `with`, and `range`
  when structure stays fixed after construction. Place widgets where output
  must change; plain template expressions do not update themselves.
- [ ] Use a Container when children can be added, removed, reordered, or replaced;
  preserve equal child definitions so unchanged Elements survive. Dirty the
  provider for list changes and retained children's tags for content changes.
- [ ] Use a JaWS Template only when rerunning that region is intended. Account
  for replacement of its contents, focus, and unsent edits when HTML changes.

### Choose widgets and their lifetime

Guide: `doc/ui/README.md#widget-lifetime-and-identity`, `doc/ui/custom.md`.

- [ ] **Use standard widgets with application bindings, getters, and templates.**
  Attribute changes, events, and focus retention usually need no custom widget.
  A JaWS Template wrapper is an exception: its updates replace only inner HTML.
- [ ] Before adding a custom UI type or overriding `JawsRender`/`JawsUpdate`,
  identify required behavior standard composition cannot provide, or a measured
  cost that justifies specialization. If needed, embed a standard widget and
  override only what is necessary.
- [ ] Construct fresh widgets per Request. Share synchronized sources and tags,
  not Elements or Requests. Follow each widget's within-Request reuse contract.
- [ ] Keep UI definitions comparable and equal to themselves. For Register,
  supply a render-independent updater and no nested JaWS widgets.

### Choose initial attributes

Guide: `doc/ui/README.md#choose-initial-attributes`.

- [ ] Put static attributes in constant template strings. For dynamic initial
  attributes, normally pass a member function returning `template.HTMLAttr`,
  just like a tag. `JawsInitialHTMLAttr` is an alternative source contract.
- [ ] Use ordinary template conditions for initial attributes on ordinary HTML.
  Initial attribute expressions are not reevaluated by a widget update.

### Choose later content and attribute updates

Guide: `doc/ui/custom.md#update-attributes-after-rendering`, `doc/bindings.md`.

- [ ] Update the existing Element where possible. Handle both directions of
  attribute and class changes; preserve unrelated classes.
- [ ] Use getters for later attribute changes. If an attribute must be correct
  in initial HTML, also supply it through an initial attribute method; getter
  commands alone are absent from initial HTML.
- [ ] Getters run initially too, and `HTMLInner` resends content on updates.
  Accept redundant commands and content unless measured cost justifies an
  override.
- [ ] In a justified `JawsUpdate` override, call the embedded updater when its
  content or value still needs updating.
- [ ] Keep getters free of application-state mutations. Register every state
  dependency used by content or attributes.

### Choose dependency tags and update scope

Guide: `doc/tags.md`.

- [ ] Use stable tags accepted by `tag.TagExpand`, preferably from member methods. An
  Element registers every state dependency affecting its content or attributes;
  these sets of dependencies may overlap.
- [ ] Standard widgets register source tags and render-parameter tags
  automatically. `JawsGetTag` and manual registration are optional; neither is
  required for tags supplied in the template. Dirtying one tag does not dirty
  other tags on its Elements. A `JawsGetTag` result must expand to the same tag
  set after its first non-nil result, including under concurrent calls.
- [ ] Use `Dirty(tag)` for matching Elements across Requests; `Dirty(elem)`
  selects that exact Element. Use Request-specific tags for local dependencies.
  Broadcast has different destination rules; consult `doc/transport.md`.

### Implement a mutation or event

Guide: `doc/bindings.md`, `doc/tags.md`, `doc/runtime.md`.

- [ ] Attach handlers to rendered widgets or JaWS Templates during rendering.
  In handlers, change state, dirty tags, and return promptly; apply DOM commands
  during rendering or updates.
- [ ] Identify affected dependencies while changing state, then dirty their tags
  after unlocking. Avoid scanning all displayed values just to discover changes
  an operation already knows.
- [ ] Dirty only dependencies whose output can change, including on repeated or
  bulk actions. Preserve reconciliation of rejected or normalized input; an
  unchanged stored value can still need a browser correction.
- [ ] Expansion is limited to 100 unique tags per call. For larger changed sets,
  dirty independent tags separately. Use a shared tag when the whole group
  needs updating, not merely because the action concerns shared state.

### Choose session, authorization, or browser state

Guide: `doc/sessions.md`, `doc/deployment.md`, `doc/ui/jsvar.md`.

- [ ] If initial rendering needs a Session, establish it before rendering and
  initialize its state atomically. Session storage supplies no dependency tags.
- [ ] If templates use `$.Auth`, configure `MakeAuth`; enforce authorization
  independently in protected operations.
- [ ] For browser-writable `JsVarStore`, validate the complete proposed value
  and caller authorization in `ClientCheck`.

### Pass content or attributes across a trust boundary

Guide: `doc/bindings.md#html-and-attribute-safety`.

- [ ] Keep user-provided strings out of raw JaWS HTML content and attribute
  parameters. Normal strings there are trusted markup; template autoescaping
  does not protect helper arguments.
- [ ] Use escaping bindings/string getters for text, or escape before returning
  `template.HTML`. Use `htmlio.Attr` with trusted names for attribute syntax
  containing untrusted values. `Element.SetAttr` instead takes an unescaped logical value.

### Verify an interaction

Guide: `doc/testing.md`, `doc/browser.md`.

- [ ] Check initial HTML, tail commands, and subsequent WebSocket output
  separately. Assert HTML semantics rather than incidental quote formatting.
- [ ] Settle processing before asserting complete output: commands, targets,
  and serialized bytes; follow the guide's collector/wait pattern. Include
  unchanged actions and bulk operations; check unrelated Elements stay quiet.
- [ ] For shared state, use two Requests and verify local state remains local.
  At the intended scale, check mutation cost as well as wire volume.
- [ ] Run applicable tests, including concurrent mutations and rendering/update
  callbacks under `go test -race`. Check browser events, keyboard operation,
  focus, and rejected input where relevant.
