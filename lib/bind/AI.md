# AI guidance for github.com/linkdata/jaws/lib/bind

See the [module-wide AI guidance](../../AI.md) before changing this package.

## Binder construction and identity

`New(l, p)` is the main adapter: it combines a locker and a pointer into a
getter, setter, HTML getter, tag getter, and event handler. `T` must be strictly
comparable even though Go's `comparable` constraint admits some interface-shaped
cases that can panic during equality. The binder's stable tag is always `p`.
Every chained binder shares the original locker and pointer, so adding hooks
must never change dirty identity.

If the supplied locker implements `RWLocker`, reads use its read lock;
otherwise `AsRWLocker` maps reads to the ordinary exclusive lock. Binder methods
are concurrency-safe only to the extent that the supplied locker is. HTML
formatting and initial-attribute hooks run while the binder lock is held.

## Hook chain rules

Each builder returns a new head wrapping the previous binder. `GetLocked` and
`SetLocked` hooks receive the previous binder and normally delegate to it while
the appropriate lock is already held. `GetHTML`, click, and context-menu hooks
receive the current binder. Do not reacquire the binder lock or call the public
locking getter/setter from a locked hook.

Lookup and event ordering are deliberately head-first:

- The newest `GetHTML` or `Format` override wins and shadows older rendering
  overrides.
- Click and context-menu hooks run newest-first and continue only when the
  result matches `jaws.ErrEventUnhandled`.
- Successful sets run success hooks newest-first, after releasing the lock, and
  stop at the first error.
- The default setter writes only a changed value and otherwise returns
  `jaws.ErrValueUnchanged`.

`Success` accepts only the four documented dynamic function signatures. Adapter
constructors such as `MakeGetter` and `MakeSetter` likewise panic on dynamic type
mismatch. These are trusted construction-time APIs, not validators for external
values.

## HTML conversion

`MakeHTMLGetter` uses the first matching conversion in this order:

1. `HTMLGetter` unchanged.
2. `template.HTML` unchanged.
3. `Binder[string]` and `Getter[string]` with escaped getter output.
4. `fmt.Stringer` with escaped `String` output.
5. Plain `string` unchanged as trusted HTML.
6. Other values formatted with `fmt.Sprint` and escaped.

Do not reorder these cases: interface overlap makes precedence observable.
Getter and Stringer adapters expose their wrapped value as an implicit tag; it
must expand to usable tag keys or intentionally return nil through
`tag.TagGetter`. Function-backed HTML/string getters snapshot only the top-level
tag slice slots. Nested containers and referenced values remain caller-owned
and must keep stable tag identity.

## Setter targets and UI integration

Writable input sources need a stable target so an Element can reconcile a
browser edit after the setter accepts, rejects, or normalizes it. `bind.New`
provides the backing pointer. A custom setter can instead be pointer-valued or
implement `JawsGetTag` with at least one usable stable key; `JawsGetTag` takes
precedence over setter identity. Tags passed as widget render parameters only
register dependencies and do not replace this setter-derived target.

Editable Number and Range reject sources without a target during rendering.
Other reusable input bases can render but cannot automatically reconcile a
rejected or normalized value without one. Keep the complete widget-side
contract in `lib/ui/AI.md` and on the exported UI symbols.

`MakeSetter` adapters for a Getter or static value still satisfy `Setter` and
return `ErrValueNotSettable`; that affects whether numeric widgets consider the
source editable. Pass a Getter directly, or use `MakeGetter`, when the intended
numeric control is read-only.

## Verification

Run `go test -race ./lib/bind` and `go test ./lib/bind` from the module root.
Preserve tests for concurrency, lock ownership, hook ordering and fallthrough,
strict-comparability failures, adapter panics, escaped/trusted HTML precedence,
stable pointer tags, formatter locking, and top-level tag-slice snapshots. Changes
to `int` or `uint` numeric behavior also require the 32-bit leg in the
[repository verification matrix](../../AI.md#repository-verification-matrix).
