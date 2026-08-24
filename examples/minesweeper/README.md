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
- Each cell constructs a small specialization that embeds the standard JaWS
  Button. The standard render path gets content, events, and the precise tag
  directly from `*cell`. The template passes the result of
  `cell.InitialAttrs()` as an ordinary render parameter, while the specialized
  update handles mutable wrapper attributes together with inner HTML.
- A separate board tag supports broad reset and terminal-state refreshes without
  widening ordinary single-cell updates.
- Every cell also registers the shared game-over field because its label and
  disabled state depend on that field directly.
- Status and statistics use text getters with exact field tags; their Span
  widgets apply HTML escaping.
- Getters derive output directly from synchronized game fields; there is no
  detached render DTO or application-owned presentation state.
- Initial cell attributes render inline with no redundant `TailHTML` fixups.
  Live updates use `data-state` and preserve the static class supplied by the
  template. The template also supplies each Button's shared board and game-over
  dependencies explicitly.
- Mutations return only tags for dependencies they changed, and event handlers
  pass those tags to `Request.Dirty`. JaWS schedules matching Elements across
  live Requests. Those batched Element updates re-read the shared game, bringing
  every already-registered matching Element to the latest state.
- The long-running server configures logging so update-time failures are
  reported rather than panicking; tests that construct JaWS directly omit the
  logger so framework-contract violations fail fast. The server also configures
  secure response headers, embedded static files, and the JaWS processing loop.
  It omits session middleware because the game is shared.
