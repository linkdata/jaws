# AI guidance for github.com/linkdata/jaws/lib/named

This is the version-specific implementation guide for package `named`. Read the
[module guidance](../../AI.md) first. Public behavior remains documented on the
exported symbols.

## Values and trust boundaries

`Bool` and `BoolArray` model named choices used by Select, Option, Checkbox,
Radio, and RadioGroup widgets. A name is a browser form value and must be a
non-empty, valid UTF-8 string without U+0000. The zero `Bool` is therefore not
ready for use; construct entries with `NewBool` or `BoolArray.Add`.

Labels use `template.HTML` and are trusted HTML. Escape user-controlled text
before constructing an entry. Keep this rule aligned with the HTML boundaries
in [bind](../bind/AI.md), [htmlio](../htmlio/AI.md), and [ui](../ui/AI.md).

## Selection and dirtying

- Each independently constructed `ui.Radio` is one boolean binding. Native
  radio grouping does not update peer Go values because the browser reports
  only the control that produced the event.
- Use a single-select `BoolArray` for `ui.Select` or a server-side radio group.
  Construct it with `NewBoolArray(false)` or use its zero value. Give every
  option a distinct name. Multi-select arrays and duplicate `Bool` names are
  unsupported by `ui.Select` and `ui.RadioGroup`.
- `Bool.JawsSet` acquires the owning array lock before the value lock, changes
  the selected value, clears peers when needed, releases value locks, and then
  dirties the affected `Bool` values and the array. Preserve that order.
- `Bool.Set` changes only one value. It neither clears peers nor dirties UI, so
  it is not a replacement for `JawsSet` in widget-driven selection.
- `BoolArray.JawsSet` matches every entry with the submitted name, while
  `Bool.JawsSet` updates only its receiver among same-name entries. In
  single-select mode, a missing name clears the selection.
- Removing a `Bool` from an array does not rewrite its fixed owner pointer.
  Follow the exported `WriteLocked` contract before using a removed value.

See [tag](../tag/AI.md) for target registration and [ui](../ui/AI.md) for the
RadioGroup and Select rendering rules.
