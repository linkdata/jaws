# AI guidance for github.com/linkdata/jaws/lib/templatereloader

This is the version-specific implementation guide for package
`templatereloader`. Read the [module guidance](../../AI.md) first. Public behavior
remains documented on the exported symbols.

## Build-dependent loading

`New` must resolve the same logical template set in both modes:

- normal builds parse `fpath` once from `fsys` with `html/template.ParseFS` and
  return a `*template.Template`;
- debug and race builds join `relpath` with `fpath`, parse from disk, and return
  a `*TemplateReloader` so edits appear without restarting the process.

Consequently, `fpath` must be valid for both `io/fs.Glob` and
`path/filepath.Glob`, and the disk tree below `relpath` must mirror the embedded
file-system layout. In normal builds `relpath` is not used.

An initial parse failure in either mode returns a true nil `TemplateLookuper`
alongside the error, not an interface containing a nil template pointer.

## Reload behavior

- The default reload interval is one second. A non-positive internal interval,
  including the zero value's interval, selects that default.
- `Lookup` uses a read-then-write-lock recheck so concurrent stale observations
  produce at most one parse for an interval.
- A failed parse is recorded by `LastError`; the last successfully parsed
  template remains active. Another attempt waits for the next interval.
- `Path` exposes the disk glob used in reload mode. Both `Path` and `LastError`
  support nil receivers according to their exported contracts.
- A zero `TemplateReloader` has no current template and `Lookup` returns nil.

Keep the error return from initial construction unwrapped: it is the only
failure class and callers need the parser detail, not another identity.

The [jawsboot guide](../../jawsboot/AI.md) describes the example integration.
Test normal and debug/race behavior separately, including last-good retention,
concurrent lookup, zero values, and path reporting.
