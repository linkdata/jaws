# AI guidance for github.com/linkdata/jaws/examples/minesweeper

See the [module-wide AI guidance](../../AI.md) before changing this example.

## Application model

The demo is a server-driven JaWS application with no custom client-side state.
One `game` is created in `run` and shared by all visitors, making the running
demo intentionally collaborative. For per-user state, wrap the page in an outer
handler that loads the user's game and invokes a freshly constructed
`ui.Handler` for that request. Put `jw.SessionMiddleware` outside that handler
when the lookup uses a JaWS Session. Do not silently change the existing
shared-game behavior.

The board shape and cell pointers are fixed after construction. Mutable game
and cell fields are protected by `game.mu`. Cell getters lock the game and read
those fields directly. Do not add a detached render DTO or parallel presentation
state: `game` remains the sole application model. `cell.Button` constructs a
fresh specialization that embeds the standard `ui.Button`; the shared pointer
is the authoritative cell, not copied UI state. Its promoted standard render
uses the cell as its getter, event source, and precise tag, while the template
passes the result of `cell.InitialAttrs()` as an ordinary parameter. Its
specialized update captures that one Button's current attributes and content
under the game lock, unlocks, and then queues Element changes. Keep state
mutations out of getter/render paths.

`run` constructs the application inline and injects only `listenAndServe`.
Preserve that copyable layout unless a production behavior requires another
seam. The long-running server must configure the JaWS logger before setup so
update-time failures are reported instead of panicking. Tests that construct
JaWS directly deliberately leave it nil so illegal tags and other
framework-contract violations fail fast. Start `Serve` before exposing the
handlers, and do not add session middleware while the application has no
session-owned state.

## Dirty targeting

- A `cell` is its own precise tag. `cell.JawsGetTag` must return only the cell.
- The template passes `cell.BoardTag` (`&g.cells`) and `cell.GameOverTag`
  (`&g.gameOver`) to every cell Button. This lets `Dirty(cell)` update one cell,
  `Dirty(&g.cells)` refresh the complete board, and `Dirty(&g.gameOver)` update
  both status and the terminal presentation of every cell.
- Do not return the shared board tag from `cell.JawsGetTag`. Tag expansion would
  turn every single-cell action into a full-board update.
- Scalar status dependencies use the addresses of the exact `game` fields.
  Each mutation appends a field tag exactly when it changes that field.
  HTML-inner widgets do not perform application-level diffing, so broad scalar
  dirtying causes needless DOM work.
- Loss, win, and any reset that changes cell state use the shared board tag.
  Normal reveals return the individual cells reached by flood fill, and flag
  toggles return only the affected cell plus changed scalar fields.

Dirty tags carry dependency identities, not rendered values. `Request.Dirty`
schedules matching Elements across live Requests, and JaWS may batch or coalesce
those updates. Each scheduled Element re-reads the authoritative game state. A
dirty event can select only dependencies an Element has already registered; each
later matching event brings every registered copy current, including any cell
whose separately locked initial attribute and content reads straddled a mutation.
Mutations must not push a second representation of the game into UI objects.

The template owns the static `cell` class. Dynamic styling uses one
`data-state` attribute, so updates preserve caller-supplied classes. This is the
deliberate reason for the Button overload: without the specialization, the
standard Button would call its HTML getter during both initial rendering and
dirty updates. Queueing wrapper changes there adds 300 redundant DOM operations
to `TailHTML`. The specialized update keeps the initial tail empty while retaining
the standard Button render path. `BenchmarkInitialPageAndTail` is the regression
guard for that measured payload improvement.

Initial attributes come from the ordinary `cell.InitialAttrs()` template
argument; the cell does not implement `JawsInitialHTMLAttr`. Do not turn that
framework hook into an update callback. Its `template.HTMLAttr` result is trusted
opening-tag syntax: by itself it identifies neither removals nor attribute
ownership, and initial duplicate handling is not equivalent to live DOM updates.
A reusable dynamic-attribute API would need an explicit ownership/diff contract,
preferably structured set/remove operations, and belongs in a separate framework
change.

`TestSingleCellDirtyStaysScopedToOneCell` guards the targeted-update invariant.
The committed `BenchmarkSingleCellDirtyFanout` measures tag expansion and
cell-Element lookup cost while intentionally excluding the separate Stats
update. Keep both when changing cell identity or tag registration.

## Domain behavior

The first reveal places mines while excluding the selected cell. Empty-cell
reveal uses an iterative stack and does not reveal flagged cells or mines.
Construction clamps the board to at least two rows and columns and clamps the
mine count to at least one and below the cell count. The game ends when a mine
is revealed or every safe cell has been revealed; both terminal states reveal
the mines and refresh the board.

Click reveals a cell. Context-menu and Shift-click events toggle its flag, so
the same semantic Button supports ordinary and modifier-assisted activation.

The trusted HTML returned by `cell.htmlLocked` consists only of fixed markup and
an integer adjacency count. Do not interpolate user-controlled content into it.

## Testing responsibilities

- Keep pure domain tests for construction bounds, first-click safety, mine
  placement, adjacency, flood fill, flags, win/loss, reset, and no-op guards.
- Keep UI integration tests on real JaWS Elements for tag registration, event
  dispatch, queued attribute updates, an empty initial TailHTML, and exact dirty
  fanout.
- Use at least two live Requests to verify collaborative narrow updates and
  board-wide refreshes.
- Keep the HTTP wiring test for templates, static assets, middleware, and route
  setup without binding a real port.
- Run `go test -race ./examples/minesweeper` and a plain
  `go test ./examples/minesweeper` from the module root. Run the benchmark with
  `-bench=SingleCellDirtyFanout -benchmem` when changing dirty-target behavior,
  and `-bench=InitialPageAndTail -benchmem` when changing initial rendering.
