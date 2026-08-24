# JaWS Minesweeper

This example is a collaborative Minesweeper board rendered entirely from
synchronized Go state. Every connected browser shares the same game, and the
page contains no application-specific JavaScript.

Run it from the module root:

```sh
go run ./examples/minesweeper
```

Then open <http://localhost:8080>. Click to reveal a cell; right-click or
Shift-click to toggle a flag.

The example demonstrates the main JaWS application patterns together:

- Template execution constructs fresh Button and Span definitions for each
  Request over the shared `*game` and `*cell` sources.
- Each cell is a semantic Button source: it provides content, event handlers,
  initial attributes, and its precise dependency tag.
- A separate board tag supports broad reset and terminal-state refreshes without
  widening ordinary single-cell updates.
- Every cell also registers the shared game-over field because its label and
  disabled state depend on that field directly.
- Status and statistics use text getters with exact field tags; their Span
  widgets apply HTML escaping.
- Getters derive output directly from synchronized game fields; there is no
  detached render DTO or application-owned presentation state.
- Mutations return only tags for dependencies they changed, and event handlers
  pass those tags to `Request.Dirty`. JaWS schedules matching Elements across
  live Requests. Those batched Element updates re-read the shared game so every
  connected browser eventually converges.
- The server configures logging, secure response headers, embedded static files,
  and the JaWS processing loop. It omits session middleware because the game is
  shared.
