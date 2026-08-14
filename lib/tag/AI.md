# AI guidance for github.com/linkdata/jaws/lib/tag

This is the canonical version-specific guide to JaWS dependency tags. Read the
[module guidance](../../AI.md) for request lifecycle and dirty dispatch. Public
behavior remains documented on the exported symbols.

## Purpose and selection

Tags associate Elements with authoritative application data or logical signals.
A tag does not observe data and does not schedule work by itself. After changing
state, dirty the keys whose rendered output changed, or use those keys as
broadcast destinations.

Prefer stable identities derived from the data being rendered:

```go
type UserData struct {
	Name string
}
type userNameTag struct{ User *UserData }
type clockTag struct{}

nameElem.Tag(user, userNameTag{User: user})
clockElem.Tag(clockTag{})

rq.Dirty(userNameTag{User: user})
rq.Dirty(user)
rq.Dirty(clockTag{})
```

Use a data pointer when output depends on the whole object, a comparable wrapper
for one independently changing part, and a named empty struct for a shared
signal. Name a tag for the dependency, not the event that changes it.

Register an item Element with both its item key and a separate group key when it
must support narrow and broad updates. Do not return the group key from the
item's own `JawsGetTag`: expanding the item for `Dirty(item)` would then include
the group and refresh every group listener.

## Expansion

`TagExpand` recursively flattens direct keys, `[]Tag`, `[]any`, and `TagGetter`
results into unique keys. Every key must be comparable at runtime and equal to
itself. Slices, maps, functions, interface values containing them, and values
containing NaN are not usable keys.

The exact types `string`, `bool`, every signed integer, `uint` through `uint64`,
`float32`, `float64`, `template.HTML`, `template.HTMLAttr`, `jid.Jid`, and
`key.Key` are rejected. `uintptr` and complex types are not on that rejection
list, although they must still be comparable and reflexive. An alias of a
rejected type is rejected; a defined type is tested by its own exact dynamic
type rather than solely by its underlying type. Use `tag.Tag("name")`, a domain
struct, or a pointer rather than a plain string.

A `TagGetter` controls expansion; it does not register an Element. It may return
a nil interface during a documented initialization phase. That result expands
to no keys and later initialization is not retroactive. After the first non-nil
result:

- every call must expand to the same key set;
- previously returned containers remain immutable and keep their meaning;
- callers may invoke it before rendering, repeatedly, or concurrently;
- the implementation synchronizes mutable state and safe publication itself.

`Element.ApplyGetter` may call `JawsGetTag` and expansion may call it again. Do
not write logic that depends on an exact call count or Request context.

## Registration and lifetime

`Request.Tag` and `Element.Tag` expand and add associations. Registration is
additive and lasts until the Element is removed or its Request ends; individual
associations cannot be withdrawn. Register known dependencies during initial
render. A dependency discovered later may be added only when it remains valid
for the Element's remaining lifetime.

Adding a tag does not affect an already selected dirty or broadcast operation.
Register first, then target the key.

`Request.TagsOf` returns the keys actually registered on an Element.
`Request.GetElements` expands its argument. `Request.HasTag` is the advanced
exact-key check: it neither expands nor validates its argument and can panic for
an invalid value.

## Dirty and broadcast targets

`Request.Dirty` and `Jaws.Dirty` both expand through `Jaws.MustTagExpand`.
An expanded non-nil `*Element` is an exact target on its owning Request; every
other key targets all matching live Requests. Ordinary tags passed through
`Request.Dirty` are therefore not scoped to that Request.

Expansion has independent limits of 10 nested expansion levels and 100 unique
tags. Exceeding either reports `ErrTooManyTags`. The count check occurs after the
new tag is appended, so a count failure can return 101 partial entries. With a
logger the partial result is applied and the failure is logged; without a logger
the call panics before dirtying. Prefer one shared group key over enumerating a
large set.

`wire.Message.Dest` follows the same distinction between request-local element
destinations and tag destinations. See [wire](../wire/AI.md).

## Build modes and verification

Race and debug builds select the full-detail tag renderer. Plain builds select
the crash-safe release renderer used in production. The comparability checks in
this package run in every build. Always test both modes from the module root:

```sh
go test -race ./...
go test ./...
```

When the race detector is unavailable, use `-tags "debug deadlock"` plus the
plain test leg. Benchmarks that change tag expansion or rendering must exercise
the affected operation and retain allocation reporting.
