# Examples

[Documentation](README.md) · [Getting started](getting-started.md) · [Testing](testing.md)

The repository includes small application examples and a runnable collaborative
Minesweeper game. Start with [Getting started](getting-started.md) for the basic
application setup, then use these examples to combine the individual APIs.

## Small applications

[`examples/example_test.go`](../examples/example_test.go) contains two complete
server functions:

- `Example` renders a range control bound to a synchronized Go value.
- `Example_secureSession` adds session cookies and secure response headers. Its
  outer `SecureHeadersMiddleware` also covers errors from `SessionMiddleware`.

Both functions start a blocking HTTP server. `go test ./examples` compiles them
but does not run them. To run one, copy its imports, template, and function into
a `main` package and rename the selected function to `main`. The minimal example
also uses the `Percent` type defined in the file.

The examples configure JaWS, register templates, start `Serve`, register HTTP
routes, and then start the HTTP server. See [Sessions](sessions.md) for
request-specific state and [Deployment](deployment.md) for HTTP configuration.

## Run Minesweeper

From the module root:

```sh
go run ./examples/minesweeper
```

Open [localhost:8080](http://localhost:8080) for a 10×10 board with 15 mines.
Click to reveal a cell; right-click
or Shift-click to toggle a flag. The first reveal is safe. Revealing a mine ends
the game; revealing every safe cell wins it. Both outcomes reveal the mines.

All visitors share one game. Open two browser windows to see an action in one
window update the other. The application uses the JaWS browser runtime without
application-specific JavaScript.

| File | Contents |
| --- | --- |
| [`main.go`](../examples/minesweeper/main.go) | Embedded assets, routes, logging, and server setup |
| [`game.go`](../examples/minesweeper/game.go) | Synchronized board state, reveals, flags, and reset |
| [`ui.go`](../examples/minesweeper/ui.go) | Cell controls, getters, and event handlers |
| [`index.html`](../examples/minesweeper/assets/ui/index.html) | Layout and dependency registration |
| [`main_test.go`](../examples/minesweeper/main_test.go) | Domain, rendering, live-update, and HTTP tests |

### Follow an update

1. The template constructs fresh UI definitions over the shared `game` and
   `cell` pointers for each Request.
2. `cell.JawsClick` reveals a cell or toggles its flag when Shift is pressed.
   `cell.JawsContextMenu` toggles the flag.
3. The game method changes state under `game.mu` and returns the affected tags.
4. The event handler dirties each returned tag through `dirtyTags`.
5. Matching Elements in live Requests update from the current game state.

Dirty tags identify dependencies; they do not carry rendered values. JaWS can
batch updates. See [Tags and updates](tags.md) for dependency registration and
[Runtime](runtime.md) for the Request lifecycle.

### Select the affected controls

Each cell Button has three dependencies:

| Dependency | Registered through | Use |
| --- | --- | --- |
| `cell.CellTag()` (the `*cell` pointer) | The Button's content source | Refresh one cell |
| `&game.cells` | The template's `.BoardTag` parameter | Refresh the board after reset or game end |
| `&game.gameOver` | The template's `.GameOverTag` parameter | Refresh terminal labels and disabled states |

Status and statistics use getters tagged with the addresses of their specific
game fields. For example, a flag action dirties the cell tag and `&game.flags`.
An ordinary reveal dirties the tags for cells reached by flood fill and changed
status fields. A terminal reveal uses the board tag.

The `dirtyTags` helper makes a separate call for each tag, so large flood fills
stay within the per-expansion limit. See [tag limits](tags.md#valid-tags-and-limits).

### Render and update a specialized Button

`cellButton` embeds `ui.Button` and supplies its own `JawsUpdate`. The standard
Button handles initial rendering, content getters, tags, and events. The cell
implements `bind.HTMLGetter` through `JawsGetHTML`, preserving its event methods
when passed as the Button's content source; see [source selection](bindings.md).
The template passes constant `class="cell"` and `.InitialAttrs` as
[initial attribute parameters](ui/README.md#choose-initial-attributes).
The custom update reads the current attributes and inner HTML under the game
lock, then queues the changes after unlocking.

Live styling uses `data-state`; updates set or remove `disabled` and update
`aria-label`. Status text uses
escaped Span content. See [UI construction](ui/README.md) and [Bindings](bindings.md)
to apply these patterns to other controls.

The game is created once in `run`, so `ui.Handler` shares it between Requests.
To build a separate game per user, load that user's game in an outer HTTP
handler and construct `ui.Handler` there. If the lookup uses a JaWS Session,
place `SessionMiddleware` outside that handler; see [Sessions](sessions.md).

### Run the example checks

```sh
go test -race ./examples/minesweeper
go test ./examples/minesweeper -run '^$' -bench 'SingleCellDirtyFanout|InitialPageAndTail' -benchmem
```

The benchmarks measure cell lookup fanout and the initial page/tail payload.
The live-update tests use the [request test harness](testing.md) with multiple
Requests sharing one game.
