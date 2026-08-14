# AI guidance for github.com/linkdata/jaws/examples/minesweeper

See the [module-wide AI guidance](../../AI.md) before changing this example.

## Application model

The demo is a server-driven JaWS application with no custom client-side state.
One `game` is created in `run` and shared by all visitors, making the running
demo intentionally collaborative. Create request- or session-owned games in
the page handler if that product behavior changes; do not silently turn the
existing shared game into per-user state.

The board shape and cell pointers are fixed after construction. Mutable game
and cell fields are protected by `game.mu`. Rendering takes an immutable
`cellView` snapshot while holding the lock, releases the lock, and then derives
trusted cell markup and queues Element presentation updates from that snapshot.
Keep state mutations out of getter/render paths.

`run` deliberately constructs the application inline and injects only
`listenAndServe`. Preserve that copyable layout unless a production behavior
requires another seam.

## Dirty targeting

- A `Cell` is its own precise tag. `Cell.JawsGetTag` must return only the cell.
- Every cell button separately registers `Cell.BoardTag`, which is `&g.cells`.
  This lets `Dirty(cell)` update one cell and `Dirty(&g.cells)` refresh the
  complete board.
- Do not return the shared board tag from `Cell.JawsGetTag`. Tag expansion would
  turn every single-cell action into a full-board update.
- Scalar status dependencies use the addresses of the exact `game` fields.
  Mutations snapshot scalar state before changing it, then `changedTags` emits
  only fields whose values differ afterward. HTML-inner widgets do not perform
  application-level diffing, so broad scalar dirtying causes needless DOM work.
- A loss, win, or reset changes many cells and uses the shared board tag. Normal
  reveals return the individual cells reached by flood fill, and flag toggles
  return only the affected cell plus changed scalar fields.

The committed `BenchmarkSingleCellDirtyFanout` guards the targeted-update
design. Keep it when changing cell identity or tag registration, and verify it
still resolves a single-cell action to one cell Element.

## Domain behavior

The first reveal places mines while excluding the selected cell. Empty-cell
reveal uses an iterative stack and does not reveal flagged cells or mines.
Construction clamps the board to at least two rows and columns and clamps the
mine count to at least one and below the cell count. The game ends when a mine
is revealed or every safe cell has been revealed; both terminal states reveal
the mines and refresh the board.

The static `template.HTML` fragments in `cellView.HTML` contain only fixed
markup plus an integer adjacency count. Do not interpolate user-controlled
content into those trusted fragments.

## Testing responsibilities

- Keep pure domain tests for construction bounds, first-click safety, mine
  placement, adjacency, flood fill, flags, win/loss, reset, and no-op guards.
- Keep UI integration tests on real JaWS Elements for tag registration, event
  dispatch, queued class/attribute updates, and exact dirty fanout.
- Keep the HTTP wiring test for templates, static assets, middleware, and route
  setup without binding a real port.
- Run `go test -race ./examples/minesweeper` and a plain
  `go test ./examples/minesweeper` from the module root. Run the benchmark with
  `-bench=SingleCellDirtyFanout -benchmem` when changing dirty-target behavior.
