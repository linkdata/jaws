# AI guidance for github.com/linkdata/jaws/examples

See the [module-wide AI guidance](../AI.md) before changing these examples.

## Purpose

This package contains compile-checked examples that show supported JaWS setup
patterns. Keep the minimal `Example` synchronized with the root README quick
start so a reader can move between them without encountering a different
lifecycle or wiring order.

The canonical setup sequence is:

1. Create a `jaws.Jaws` and arrange to close it.
2. Configure instance fields such as `Logger` before exposing handlers.
3. Parse templates and add their `TemplateLookuper` to the JaWS instance.
4. Start `Serve` before relying on dirtying or broadcasts.
5. Mount the `GET /jaws/` route on the selected mux.
6. Construct request-scoped UI values and mount the page handler.
7. Start the HTTP server.

Examples that add sessions or secure headers should build on that sequence,
not replace it with a second basic setup pattern. Keep detailed lifecycle,
security, and widget contracts in the package that owns them; this package
demonstrates how those contracts fit together.

## Example conventions

- Examples must compile as part of `go test`. Use real public APIs and handle
  returned errors unless the API explicitly makes an error impossible for the
  demonstrated input.
- The server examples intentionally block in `http.ListenAndServe`. They have no
  `Output:` comment, so `go test` compile-checks but does not execute them.
- Keep examples short enough to copy. A production concern belongs here only
  when omitting it would teach an unsafe default; link to the owning package for
  the full contract.
- If the README quick start changes, update `Example` in the same change and
  compare imports, template helpers, initialization order, routes, and cleanup.
  `Example_secureSession` may contain the additional middleware needed for its
  specific subject.

## Verification

Run `go test ./examples` from the module root. Also inspect the rendered example
with `go doc github.com/linkdata/jaws/examples` and compare the minimal example
against the root README quick start.
