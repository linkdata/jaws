# AI guidance for github.com/linkdata/jaws/jawstest

See the [module-wide AI guidance](../AI.md) before changing this package.

## Package boundary

`jawstest` is an importable, low-level harness for a real `jaws.Request`
message-processing loop. It lives outside package `jaws` so consumers of the
production package do not acquire its `net/http/httptest` dependency. The
harness reaches the loop only through the exported `jaws.Jaws.TestServe` hook;
it is not a second implementation of request processing.

Keep higher-level rendering assertions in the package under test. `Recorder`
starts with `Cache-Control: no-store`; the harness writes no response body.
Tests may render into it. `BodyString` trims the body; `BodyHTML` treats it as
trusted test content.

## Harness lifecycle

1. Create a `jaws.Jaws` and start `Serve`; `TestServe` requires the processing
   loop to be running.
2. Construct the harness with `NewTestRequest`. A nil HTTP request means a
   bodyless `GET /` request. Construction creates and claims one real Request.
3. Wait for `ReadyCh` before depending on the loop.
4. Send browser-originated frames on `InCh`, read server frames from `OutCh`,
   and inject broadcasts on `BcastCh`.
5. Drain `OutCh` whenever the test can produce output. Its buffer is finite; a
   full output channel stalls the request loop and can prevent shutdown.
6. Call `Close` to close the input side, then wait for `DoneCh`. `Close` is
   idempotent but does not itself wait.

If a test continuously drains output in a goroutine, terminate that goroutine
from `DoneCh` and wait for it before the test returns. Do not close output or
broadcast channels from test code; `Close` owns only the inbound channel.

Use `NewTestRequestWithPanic` only when the test needs to observe an expected
request-loop panic. Its callback runs on the loop goroutine and receives either
the recovered value or nil; `DoneCh` closes only after the callback returns or
unwinds. The ordinary constructor re-panics non-nil values so unexpected loop
panics remain visible.

## Maintenance and tests

Preserve construction failure as an immediate panic when a Request cannot be
created or claimed. The `newRequest` package seam exists only to exercise that
failure path; production `Jaws.NewRequest` does not return nil while open.

Run `go test -race ./jawstest` and `go test ./jawstest` from the module root.
Keep coverage for channel directions, readiness, close idempotence, output
draining, explicit requests, panic delivery, failed claims, and Request cleanup.
