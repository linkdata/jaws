# AI guidance for github.com/linkdata/jaws/lib/what

This is the version-specific command and event guide for package `what`. Read
the [module guidance](../../AI.md) and [wire framing guide](../wire/AI.md) for the
surrounding protocol.

## Vocabulary

`What` identifies either a command or a browser-originated event. Its zero value
is `Invalid`. The internal separator divides request-wide commands from values
associated with an Element; it is neither valid nor representable on the wire.
`Parse` is exact and case-sensitive, except that an empty field denotes `Update`.

Request-wide commands are `Update`, `Reload`, `Redirect`, `Alert`, `Order`, and
`Call`. Element-associated commands include `Set`, `Inner`, `Delete`, `Replace`,
`Remove`, `Insert`, `Append`, attribute/class changes, and `Value`. Input events
are `Input`, `Click`, and `ContextMenu`.

Important server-to-browser payload meanings:

- `Update` is an internal tag-targeted trigger: Request handling updates matching
  Elements rather than forwarding it to the browser.
- `Reload` ignores Data. `Redirect` carries a URL validated by the root package.
  `Alert` is the escaped level, LF, and escaped message. `Order` is a
  space-separated Jid list. These commands are page-global.
- `Call` and `Set` use `path=json`. Request-scoped `Call` has an empty Jid;
  Element-scoped `Call` and every `Set` identify an Element.
- `Inner`, `Replace`, and `Append` carry trusted HTML. `Delete` needs no Data.
  `Remove` identifies a direct child Jid. `Insert` is a child Jid or nonnegative
  child index, LF, and trusted HTML.
- `SAttr` is an attribute name, LF, and unescaped logical value. `RAttr` carries
  the name. `SClass` and `RClass` carry one class. `Value` carries textual live
  control state rather than an HTML attribute value.

Browser-to-server `Input` carries the control's textual value and invokes its
`JawsInput` handler. A browser-originated `Set` likewise invokes `JawsInput` on
the binding Element with `path=json`. `Click` and `ContextMenu` carry
coordinates, modifier-key state, the nearest name, and any managed ancestor Jids
used for event routing. Browser-originated `Remove` is a cleanup acknowledgement:
the record Jid identifies the parent/container and Data is a tab-separated list
of removed managed descendant Jids. The server removes only Elements known to
that Request.

`Hook` is a test-only synchronous event. The browser never sends it and inbound
client messages never dispatch it. A broadcast Hook lets a test invoke an
Element handler without the client round trip; the handler must not emit its own
messages, and an error is returned to the client as an Alert.

When adding or reordering constants, preserve the separator-based predicates,
generated string table, parser behavior, browser implementation, dispatch code,
and protocol tests. Run `go generate ./...` and confirm the generated file is
clean. See [assets](../assets/AI.md) for browser behavior, [ui](../ui/AI.md) for
widget event choices, and [jawstest](../../jawstest/AI.md) for loop tests.
