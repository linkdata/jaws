# AI guidance for github.com/linkdata/jaws/lib/wire

This is the canonical version-specific guide to the JaWS wire representation.
Read the [module guidance](../../AI.md) for Request lifecycle and
[what](../what/AI.md) for command semantics. Public behavior remains documented
on the exported symbols.

## Message layers

`Message` is the in-process dispatch record: a command, payload, and destination.
`WsMsg` is one browser protocol record. Destination resolution belongs to the
root and tag layers; framing must not reinterpret it:

- a nil `Message.Dest` selects every active Request;
- a nonzero `key.Key` selects the matching active Request, while a zero key is
  dropped;
- other destinations are expanded as tags or tag lists, and a non-nil
  `*jaws.Element` is an exact request-local target;
- built-in strings and bare Jids are not tag destinations.

Page-global commands emit one Jid-zero record after Request selection. `Call`
does the same for a nil or request-key destination; tag destinations resolve to
Elements and emit their Jids.

## Record framing

Each record is:

```text
What<TAB>Jid<TAB>Data<LF>
```

A WebSocket text message may contain several records. The read loop splits them
on LF, keeps order, validates each independently, and skips malformed records
without discarding valid siblings.

For commands other than `Set` and `Call`, `WsMsg.Append` writes Data as a JSON
string accepted by browser `JSON.parse`. `Parse` first uses `strconv.Unquote`,
falls back to JSON decoding for browser-valid strings such as lone UTF-16
surrogates, and sanitizes the result as valid UTF-8. `AppendJSONQuote` stays in
the overlap of both grammars and deliberately avoids Go-only escapes.

`Set` and `Call` carry Data verbatim as `path=json`. The complete verbatim Data,
including JSON, must contain no raw tab or LF byte because those delimit fields
and records. The path/function portion additionally permits no carriage return
or `=` delimiter. Inbound tabs truncate the best-effort payload at the field
boundary. Root `JsCall` normalizes the function path and compacts or escapes JSON
before it reaches this package.

`WsMsg.Append` panics for a negative Jid. Zero is encoded as an empty Jid field;
positive values use the canonical [jid](../jid/AI.md) form.

## Transport loops

- Every inbound WebSocket text message is limited to 32 KiB by the root Request
  handler. The client does not chunk inputs, JsVar writes, click data, or removal
  reports. Validation above this layer cannot enforce the transport limit.
- `ReadLoop` runs the socket reader concurrently with keepalive pings, pauses
  read-idle accounting while delivering an already-read message, and restarts
  the idle interval after incoming data or a successful ping.
- A message processed while a ping is pending makes that ping failure obsolete.
  Cancellation and normal shutdown do not become a Request error.
- `WriteLoop` coalesces consecutive queued records until its 32 KiB flush
  threshold is reached. Because it appends whole records, a batch can exceed the
  threshold by one record. Every WebSocket write gets a separate positive
  deadline; the loop closes the socket on exit and reports only failures not
  caused by cancellation or shutdown.
- Always close the writer and join its close error with the write error.

The browser implementation is described in [assets](../assets/AI.md). Widget
payload limits and JsVar representation constraints live in [ui](../ui/AI.md).

## Verification

Protocol changes need round-trip, malformed-record isolation, browser grammar,
invalid UTF-8, delimiter, batching, cancellation, ping, and timeout coverage.
Keep fuzz tests for the quoting/parsing boundary. Performance work in quoting,
parsing, or batching requires a committed benchmark with allocation reporting.
