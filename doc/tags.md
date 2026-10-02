# Dependency tags and updates

[Documentation](README.md) · [Bindings](bindings.md) · [Widgets and templates](ui/README.md) · [Runtime](runtime.md)

A tag identifies an application dependency. An Element registers the tags for
all state that affects its output: content, permissions, highlighting, enabled
state, or anything else. Dirtying any of those tags schedules that Element for
an update. Tags do not watch values; application code dirties the tags when the
corresponding state changes.

## Choose a tag

Use a stable identity for each dependency:

| Dependency | Tag |
| --- | --- |
| A bound field | Its address, such as `&state.Name`. `bind.New` supplies this automatically. |
| An object's dependency | A member function such as `myUser.PermissionsTag()`, returning a stable pointer or comparable value. |
| A shared signal | A named empty struct or `tag.Tag("clock")`. |

Tags can be shared by overlapping sets of Elements. A shared tag can select a
whole group, but has no special group behavior. An Element can register multiple
tags; registering them together does not connect their dirty state.

**Identify affected dependencies when changing state, then dirty their tags
after unlocking.** A shared state
change can affect only a subset of Elements; dirty tags selecting that subset.
A summary needs updating only when its content or attributes can change.
`HTMLInner` widgets resend content on every update, and `SetInner`, `SetAttr`,
and `RemoveAttr` queue commands without comparing previous values. Broad dirtying
produces traffic even for unchanged controls.

## Register dependencies

Standard widgets automatically register tags exposed by their source and tags
passed as render parameters. No separate `elem.Tag(...)` call is needed.
Use `with` to select an application object and call its tag methods directly:

```gotemplate
{{with .Dot}}
  {{with .CurrentUser}}
    {{$.Span .NameUI `class="username"` .NameTag .PermissionsTag .HighlightTag}}
  {{end}}
{{end}}
```

The outer `with` selects the application data; the inner one selects its current
user. `$` still provides the JaWS helpers. `NameUI` returns an HTML getter, while
the three tag parameters register the dependencies for this Span. Neither
`JawsGetTag` on the user nor a manual registration call is needed here.
See the [matching NameUI method](bindings.md#compute-content-and-attributes).

For manual registration, use `Element.Tag` or `Request.Tag`; both expand their
arguments into tags. Registration lasts until the Element is removed or its
Request ends; individual associations cannot be withdrawn.

Register known dependencies during initial rendering. A dependency added later
must remain valid for the Element's remaining lifetime. Adding a tag does not
schedule an update or change an already selected dirty or broadcast operation.

`Request.TagsOf` returns the tags registered for an Element.
`Request.GetElements` expands its argument into tags and finds matching Elements
in that Request. `Request.HasTag` checks whether one Element has one tag without
expansion or validation; an invalid tag can panic.

## Update one control or shared state

`jw.Dirty(...)`, `rq.Dirty(...)`, and `elem.Dirty(...)` use the same dispatcher:

- A non-nil `*jaws.Element` selects exactly that Element on its owning Request.
- Every other expanded tag selects matching Elements across all live Requests
  on the Jaws instance.

Calling `rq.Dirty(&state.Name)` can therefore update several users or tabs.
Use `rq.Dirty(elem)` for a correction local to one browser control. Use a
request-specific tag when several controls on one page depend on the same
request-local state. [Sessions](sessions.md) describes user-scoped state.

Pending Requests retain ordinary dirty tags until processing starts. Matching
Elements registered later during initial rendering therefore catch up when the
page connects. This applies to dirty-driven widget updates, not plain template
output or arbitrary broadcasts.

[Broadcast destinations](transport.md#route-an-in-process-message) select
Requests or registered tags; only `Dirty` has exact-Element selection.
Start the JaWS serving loop before
dirtying or broadcasting; see [runtime](runtime.md).

## Expose tags from a source

A source can expose tags itself instead of supplying them in every template.
For example, the same `User` can implement `bind.HTMLGetter` and `tag.TagGetter`:

```go
func (u *User) JawsGetHTML(e *jaws.Element) template.HTML {
    return u.NameUI().JawsGetHTML(e)
}

func (u *User) JawsGetTag() any {
    return []any{u.NameTag(), u.PermissionsTag(), u.HighlightTag()}
}
```

Rendering that source registers all three tags automatically:

```gotemplate
{{with .Dot.CurrentUser}}{{$.Span .}}{{end}}
```

```go
// After changing this user's permissions:
jw.Dirty(myUser.PermissionsTag())

// After changing this user's highlighting:
jw.Dirty(myUser.HighlightTag())
```

Each dirty tag updates every Element registered for it. Those Elements' other
tags are not dirtied. Passing a TagGetter to `Dirty` instead expands and dirties
all the tags it exposes.

`tag.TagExpand(source)` recursively flattens `[]any`, `[]tag.Tag`, and
`JawsGetTag` results, returning unique, validated tags and any error.

A TagGetter may return nil during a documented initialization phase. After its
first non-nil result, every call must expand to the same tag set. Previously
returned containers must remain unchanged in meaning. Calls can occur before
rendering, repeatedly, and concurrently; synchronize any mutable state and
publish returned containers safely. Initialization does not update earlier
registrations.

## Valid tags and limits

Every expanded tag must be comparable at runtime and equal to itself. Slices,
maps, functions, interface values containing them, and values containing NaN
cannot be tags. Keep mutable collections behind stable pointers.

Plain strings, booleans, signed integers, unsigned integers except `uintptr`,
floating-point values, `template.HTML`, `template.HTMLAttr`, `jid.Jid`, and
`key.Key` are rejected. These checks match exact dynamic types: a defined domain
type can be accepted even when its underlying type is on that list. Use
`tag.Tag("name")` instead of a plain string.

Expansion allows at most 10 nested levels and 100 unique tags. `Jaws.MustTagExpand`,
used by `Dirty` and `Tag`, logs expansion errors and returns the partial result;
without a logger, it panics. This limit applies per expansion, not to the number of Elements registered
for a tag. For a large changed set, such as a bulk edit, dirty each independent
tag in a separate call. Use a shared tag when the whole group needs updating.

See [TagExpand and its accepted inputs](../lib/tag/tag.go),
[TagGetter's contract](../lib/tag/taggetter.go), and
[runnable tag examples](../lib/tag/example_test.go).
