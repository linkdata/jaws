# AI guidance for github.com/linkdata/jaws/lib/assets

See the [module-wide AI guidance](../../AI.md) before changing this package.

## Package role and trust boundary

This package embeds the thin JaWS browser client and stylesheet and contains
helpers used while generating page metadata. The server is authoritative. The
client attaches event forwarding to managed `Jid.*` nodes, applies explicit DOM
commands, tracks live `JsVar` name routes, and reconnects after transport loss;
it does not replicate application state or business rules.

Server-sent HTML is intentionally inserted as HTML. The client must not escape
it because trusted widget markup and replacements need full DOM semantics.
Therefore escaping belongs on the server before untrusted data enters
`template.HTML` or the raw-string HTML paths. Preserve the guards that refuse to
set or remove the managed `id` attribute and that accept only canonical positive
`Jid.*` element IDs.

## Browser events and connection gating

- Input, click, context-menu, and `jawsVar` writes are sent only when the
  WebSocket is open. Interaction while connecting or disconnected is ignored,
  not queued or replayed. Applications that need a readiness gate must render
  controls disabled or inert and remove that gate after connection through the
  server-side Request API.
- Inputs, selects, and textareas use native input routing. Numeric inputs marked
  for JaWS use `change`; other managed inputs use `input`. Click and
  context-menu forwarding ignores origins inside inputs, selects, textareas, or
  options so ancestor handlers do not compete with native control behavior.
- Click-like payloads include coordinates, modifier state, the element name,
  and the canonical managed ancestor route. The server owns validation of
  values such as non-finite coordinates.
- DOM replacement/removal reports disappeared managed descendants to the
  server. Direct-child validation for insert/remove positions prevents an
  unrelated same-ID node elsewhere in the page from becoming a target.
- Each command in a batched frame is isolated. A failing DOM command is logged
  and later commands in the same frame still run.

The reconnect path observes a five-second grace period after a WebSocket
failure. It neither shows the connection-lost indicator nor probes
`/jaws/.ping` before that period elapses. It reloads only after the navigation
is old enough. Pagehide invalidates active and reconnecting state, and a bfcache
pageshow reloads. Keep reconnect constants and behavior covered by the
JavaScript runtime tests.

## Browser helpers

`jawsVar` reads or writes an application-owned global and sends a `Set` frame
for every live binding registered under its top-level name. Non-empty dotted
components are verbatim JavaScript property names: on an array, `"1"` addresses
an element, while `"01"` creates a side property omitted by JSON serialization
and rejected by generic Go array and slice bindings. Exact `__proto__` path
components are rejected, and the routing table remains a `Map` so names cannot
mutate an object prototype. Full `JsVar` authority, validation, and
synchronization rules belong to `lib/ui/AI.md`.

Value updates avoid writes when possible and preserve text selection when a
textual value changes by insertion or removal. Managed native form reset is not
implemented: it does not generate the per-control events JaWS transports.

`JavascriptText` and `JawsCSS` are immutable embedded strings. `ISO8601` is the
browser date-input format. `DefaultCookieName` is computed once at package
initialization; `MakeCookieName` keeps ASCII letters and digits from the
extension-stripped executable base name and falls back to `jaws`.

## Resource classification

`PreloadHTML` delegates URL classification to `secureheaders.InferResource`.
Matched `.js` resources become deferred classic scripts, `.css` resources
become stylesheets, and resources classified exclusively as images or fonts
become preloads. Fonts use anonymous CORS. Module/MIME-only scripts and styles,
generic fetch resources, and unrecognized URLs are omitted.

A resource whose base name starts with `favicon` is returned and emitted as the
icon only when classified as an image. The last qualifying favicon wins.
Attribute values are escaped through `htmlio`; nevertheless URLs are trusted
application configuration because emitted scripts execute and origins affect
the page security policy.

## Verification

Run `JAWS_REQUIRE_NODE=1 go test -race ./lib/assets` and
`JAWS_REQUIRE_NODE=1 go test ./lib/assets` from the module root. Requiring Node
prevents the browser-client behavior suite from silently skipping. Those tests
cover event routing, connection gating, DOM mutation, reconnection, `JsVar`
fanout, prototype safety, and batch isolation. Keep
`BenchmarkJawsJSMessageDispatch` when changing the command dispatcher. Resource
tests should assert generated markup and classification, not hashes of embedded
repository files.
