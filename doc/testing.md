# Testing applications

[Documentation](README.md) · [Examples](examples.md) · [Development](development.md)

Use ordinary Go tests for application state and `net/http/httptest` for HTTP
handlers. For live JaWS updates, `github.com/linkdata/jaws/jawstest` provides a
real Request processing loop with channels in place of the WebSocket transport.

## Start and stop a test Request

Create a JaWS instance, start `Serve`, and pass it to
`jawstest.NewTestRequest`. A nil HTTP request creates a bodyless `GET /`.
The constructor creates and claims the Request. Wait for `ReadyCh` before
driving the loop.

The harness bypasses the WebSocket upgrade and connection callbacks. Test
`ConnectFn` and `ConnectHandler` logic separately; use an integration test for
the complete connection sequence.

This is the lifecycle used by the
[runnable harness example](../jawstest/example_test.go):

```go
jw, err := jaws.New()
if err != nil {
	panic(err)
}
defer jw.Close()
go jw.Serve()

tr := jawstest.NewTestRequest(jw, nil)
<-tr.ReadyCh

// Exercise the Request here, reading OutCh while it can produce output.

tr.Close()
for range tr.OutCh {
}
<-tr.DoneCh
```

In a test, register this cleanup immediately after constructing the harness.
The Request cleanup must finish before `jw.Close`. `Close` is idempotent and
closes only `InCh`; it does not wait. Drain `OutCh` until it closes, then wait
for `DoneCh`. Do not close `BcastCh` from test code.

## Drive events and inspect updates

`TestRequest` embeds `*jaws.Request`, so it exposes methods such as `NewElement`
and `JawsKeyString` alongside these channels:

| Channel | Use |
| --- | --- |
| `InCh` | Send browser-originated `wire.WsMsg` records |
| `OutCh` | Read server-originated `wire.WsMsg` records |
| `BcastCh` | Inject `wire.Message` broadcasts into this Request |
| `ReadyCh` | Wait until the loop starts |
| `DoneCh` | Wait until the loop and cleanup finish |

Keep reading `OutCh` while producing output: its buffer is finite, and a full
buffer blocks the loop, including shutdown. If a drain goroutine owns `OutCh`,
wait for it to finish before returning from the test. Add a timeout to test
waits so a failed update produces a test failure instead of a stalled suite.

Inside a test using the lifecycle above, this renders two bound Elements,
delivers browser input, and checks the resulting display update. It uses
`sync`, `time`, and the JaWS `bind`, `tag`, `ui`, `what`, and `wire` packages:

```go
var mu sync.Mutex
value := "before"
binding := bind.New(&mu, &value)
rw := ui.RequestWriter{Request: tr.Request, Writer: tr.Recorder}
inputTag, labelTag := tag.Tag("input"), tag.Tag("label")
if err := rw.NewUI(ui.NewText(binding), inputTag); err != nil {
    t.Fatal(err)
}
if err := rw.NewUI(ui.NewSpan(binding), labelTag); err != nil {
    t.Fatal(err)
}
input := tr.GetElements(inputTag)[0]
label := tr.GetElements(labelTag)[0]
select {
case tr.InCh <- wire.WsMsg{Jid: input.Jid(), What: what.Input, Data: "after"}:
case <-time.After(time.Second):
    t.Fatal("input timed out")
}
select {
case msg, ok := <-tr.OutCh:
    want := wire.WsMsg{Jid: label.Jid(), What: what.Inner, Data: "after"}
    if !ok || msg != want {
        t.Fatalf("update = %+v, channel open=%v; want %+v", msg, ok, want)
    }
case <-time.After(time.Second):
    t.Fatal("update timed out")
}
```

The Text control already holds the accepted browser value, so it needs no
value echo; the dependent Span receives the update.

To test a button Element's handler directly, construct click data with
`jaws.Click.String`:

```go
event := wire.WsMsg{
    Jid:  button.Jid(),
    What: what.Click,
    Data: (jaws.Click{Name: "increment"}).String(),
}
```

Send `event` on `tr.InCh`, using the same bounded send as the input example.
Choose the name and modifier fields for the application action being tested.
This bypasses the browser's ancestor routing and name lookup; check those
through the [browser client](browser.md#events-and-connection-readiness).

## Render application UI in the harness

Register the application's templates with `jw.AddTemplateLookuper`, then render
the relevant partial into the existing test Request:

```go
rw := ui.RequestWriter{Request: tr.Request, Writer: tr.Recorder}
if err := rw.Template("main", "content", app); err != nil {
    t.Fatal(err)
}
```

Here `content` is a registered partial template, and `app` is its application
data. Retrieve its Elements with `tr.GetElements` using their source or
render-parameter tags. `ui.Handler` creates a separate Request, so use
it with an ordinary `httptest.ResponseRecorder` for full-document and routing
tests, rather than to render controls into `tr`.

For a shared-state test, create two TestRequests on the same `jw`, wait for both
to become ready, and render both with the same synchronized objects or binders.
Send an event through one Request and collect output from both. Each page needs
fresh widgets; shared [dependency tags](tags.md) let one change update both pages.
Look up expected Elements separately in each Request: Jids are request-local.

## Assert update scope

Render the full collection when testing whether an edit updates unrelated
items. Receiving one expected record, completing an `InCh` send, or finding
`OutCh` empty does not prove processing has finished.

For deterministic tests without network I/O, wrap the whole lifecycle in
`synctest.Test(t, func(t *testing.T) { ... })` from `testing/synctest`. Create and
close `jw`, the test Requests, and all collectors inside that callback. Give
each Request one collector as its sole `OutCh` reader; this also permits output
larger than the channel buffer. Use this cleanup instead of draining `OutCh`
again on the test goroutine:

```go
var records []wire.WsMsg
drained := make(chan struct{})
go func() {
    defer close(drained)
    for msg := range tr.OutCh {
        records = append(records, msg)
    }
}()
defer func() {
    tr.Close()
    <-tr.DoneCh
    <-drained
}()

synctest.Wait()
time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
synctest.Wait()
records = nil
tr.InCh <- event
synctest.Wait() // Let the synchronous event handler finish.
time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
synctest.Wait() // Let the dirty pass and collector finish.
// Inspect records here, while the other goroutines are blocked.
```

Queued output wakes the Request automatically; `synctest.Wait` supplies
synchronization. Fake time then crosses the normal dirty-update interval without
a real-time delay. This settles a synchronous handler followed by one dirty pass.
Advance application timers or release application-owned gates explicitly when an
action schedules additional work.

After the final wait, compare the complete collected records with the expected
commands and targets, including the absence of unrelated targets. Count
serialized payload bytes with `len(msg.Append(nil))` to detect excessive output.
With two Requests, start both collectors before sending the event and inspect
both collections after the same waits. Do not read or clear a collector's slice
before a wait establishes that its goroutine is blocked.

The [harness API](../jawstest/jawstest.go) defines the channel and shutdown
contracts.

## Check rendered output

`TestRequest.Recorder` is an `httptest.ResponseRecorder` with
`Cache-Control: no-store`. The harness leaves its body empty; render the UI
under test into it.

`BodyString()` returns the recorded body with surrounding whitespace removed.
`BodyHTML()` returns the same body as trusted `template.HTML`. Use it only with
content the test controls; it does not sanitize HTML.

For complete HTTP routing tests, construct a handler and call `ServeHTTP` with
an `httptest.NewRecorder` and `httptest.NewRequest`. Check page markup, response
headers, static resources, and JaWS routes without binding a network port.

## Observe an expected panic

`NewTestRequest` re-panics unexpected request-loop panics on the loop goroutine.
Use `NewTestRequestWithPanic` when a panic is the behavior under test. Its
callback runs on the loop goroutine with the recovered value, or nil after a
normal exit. `DoneCh` closes after the callback returns or unwinds.

Pass the value through a channel or wait for `DoneCh` before inspecting shared
callback results. The constructor itself panics if the Request cannot be
claimed, including after `Jaws.Close`, or if the JaWS processing loop is not
running.

Run the harness checks with `go test -race ./jawstest`. See
[Development](development.md) for repository-wide checks.
