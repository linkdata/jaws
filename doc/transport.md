# Transport and identifiers

[Documentation](README.md) · [Runtime](runtime.md) · [Browser](browser.md) · [Tags](tags.md)

Normal applications use `Request`, `Element`, and `Jaws` methods to update the
browser. The `wire`, `what`, `jid`, and `key` packages expose the underlying
records and identifiers for integration and diagnostics.

## Identify Requests and Elements

`jid.Jid` identifies an Element within one Request. Equal numbers from different
Requests identify different Elements. Positive values render as `Jid.1`,
`Jid.2`, and so on. Zero identifies the whole Request and renders as an empty
string. Negative values are invalid; the parsers return `jid.Invalid`, exactly
`Jid(-1)`, on failure.

`jid.ParseString` accepts an empty string or a canonical positive `Jid.<decimal>`
ID. Signs, leading zeroes, and overflow are rejected. `jid.ParseInt` instead
accepts nonnegative integers in the forms supported by `strconv.ParseInt` in
base 10, including `+1` and `01`. HTML-writing helpers emit IDs only for positive
values. See the [identifier API](../lib/jid/jid.go).

`key.Key` encodes Request and session keys as lowercase base-32 text. Zero is
invalid and encodes as an empty string. Parsing is case-insensitive and separates
the first slash from the key prefix:

```go
k, tail := key.Parse("2/noscript") // k == 2, tail == "/noscript"
```

The tail is preserved even when the prefix is invalid. Key parsing only decodes
an integer; Request lookup, client binding, and authorization belong to `jaws`.
See the [key API](../lib/key/key.go) and [Request lifecycle](runtime.md).

## Route an in-process message

`wire.Message` contains `What`, `Data`, and a `Dest` used by `Jaws.Broadcast`.

| Destination | Recipients |
| --- | --- |
| `nil` | Every active Request |
| Nonzero `key.Key` | The matching active Request |
| Zero `key.Key` | None |
| Tag or tag list | Elements registered for those tags |

Plain strings and bare Jids are not legal tag destinations. Use
[dependency tags](tags.md) or Element methods. Page-global commands produce one
record per selected Request, with Jid zero. `Call` also uses Jid zero for nil or
Request-key destinations; tag destinations produce Element Jids.
Element commands such as `Inner` and `SAttr` require tag destinations; nil and
Request keys select no Elements. `Broadcast` has no exact-Element special case;
use Element methods such as `JsCall` for one Element.
`Update` runs server-side dirty processing and is not sent to the browser.

The JaWS processing loop must run before broadcasting. `Replace` and `Remove`
cannot be broadcast; use `Element.Replace`, `Element.Remove`, or `Jaws.Delete`
as appropriate. See [broadcast methods](../broadcast.go).

## WebSocket records

Each `wire.WsMsg` is one LF-terminated record:

```text
What<TAB>Jid<TAB>Data<LF>
```

A WebSocket text message can contain several records. The server preserves
valid-record order and skips malformed records independently. Names are
case-sensitive. An empty `What` field parses as `Update`; an empty Jid field is
the whole-Request ID.

`WsMsg.Append` JSON-quotes `Data` for commands other than `JsVar` and `Call`.
For these ordinary commands, `wire.Parse` decodes quote-prefixed data and accepts
unquoted data verbatim. It accepts both Go-quoted strings and browser JSON
strings, including lone-surrogate escapes that decode to U+FFFD. `Parse` removes
invalid UTF-8 bytes from accepted data.

`JsVar` and `Call` carry `path=json` verbatim. A server `JsVar` patch can use
`path=` to delete an object property. Verbatim data must contain no raw tab or LF;
the path also excludes carriage returns and `=`. Inbound `JsVar` and `Call` data
ends at the first tab. Use the public `JsCall` and JavaScript store APIs to
construct these payloads. Appending a negative Jid panics.

See [record formatting and parsing](../lib/wire/wsmsg.go) and the
[command vocabulary](../lib/what/what.go).

## Command payloads

These are logical payloads before wire quoting:

| Command | Payload |
| --- | --- |
| `Reload` | Ignored |
| `Redirect` | URL validated by the root package |
| `Alert` | Escaped level, LF, escaped message |
| `Order` | Space-separated Jids |
| `Call` | Function path, `=`, JSON argument |
| `JsVar` | Store-relative path, `=`, JSON value; empty JSON deletes an object property on the browser |
| `Inner`, `Replace`, `Append` | Trusted HTML |
| `Delete` | None |
| `Remove` | Direct child Jid |
| `Insert` | Direct child Jid or canonical nonnegative child index, LF, trusted HTML |
| `SAttr` | Attribute name, LF, unescaped logical value |
| `RAttr` | Attribute name |
| `SClass`, `RClass` | One class name |
| `Value` | Live control value, not an HTML attribute value |

Browser `Input` and `JsVar` events invoke `JawsInput` on their Element. `Input`
carries the control value. `JsVar` carries a `path=json` proposal. Click and
context-menu events carry coordinates, modifier state, the nearest name, and
the managed ancestor route. Browser `Remove` reports removed managed descendant
Jids, separated by tabs, with the container Jid in the record. Only Elements
known to the Request are removed from server bookkeeping.

`Hook` is a synchronous test event. The browser never sends it, and inbound
messages do not dispatch it. A broadcast Hook invokes a matching handler; that
handler must not send its own messages. Its returned error becomes an alert.

## Limits and connection loops

Each inbound WebSocket message is limited to 32 KiB. An oversized message closes
the Request connection. The resulting error is retained as the Request's
cancellation cause and passed to `Jaws.Log`. The client does not split input,
JavaScript proposals, click data, or removal reports across messages.

`wire.ReadLoop` sends keepalive pings when a read remains idle. Parsing and
delivery time do not count as read-idle time. Incoming data or a successful ping
restarts the interval. Data received while a ping is pending makes a failure
of that ping irrelevant to the connection.

`wire.WriteLoop` combines queued records into a text message until reaching its
32 KiB flush threshold. It appends whole records, so a batch can exceed the
threshold by one record. Each write has its own positive timeout. The loop
closes the socket on exit. Cancellation and normal shutdown are not reported as
transport failures. See [transport loops](../lib/wire/wsio.go) and
[browser recovery](browser.md#connection-loss).
