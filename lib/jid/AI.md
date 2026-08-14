# AI guidance for github.com/linkdata/jaws/lib/jid

See the [module-wide AI guidance](../../AI.md) before changing this package.

## Identifier model

A `Jid` identifies a managed Element only within one active JaWS Request.
Equal numeric Jids in different Requests do not identify the same Element.
Positive values render as canonical HTML IDs using `Jid.<base-10>`.

`Jid(0)` is valid and represents the whole Request, but it has no HTML ID text.
Negative values are invalid. `Invalid` is the canonical parser failure and is
exactly `Jid(-1)`; manually constructed values below zero are also rejected by
`IsValid` but are not equal to `Invalid`.

## Parsing and rendering

- `ParseString("")` returns the valid whole-request Jid zero.
- Non-empty `ParseString` input must be canonical: exact `Jid.` prefix followed
  by a positive base-10 integer with no sign or leading zero. Overflow and all
  other forms return `Invalid`.
- `ParseInt` is a numeric parser rather than an HTML-ID parser. It accepts the
  forms accepted by `strconv.ParseInt` in base 10, including `+1`, `01`, and
  `-0`, as long as the decoded value is non-negative.
- `String`, `Append`, and `AppendInt` emit content only for positive values.
  `AppendQuote` always emits quotes, with empty content for zero or negative
  values.
- `AppendStartTagAttr` writes a trusted start-tag name and adds an `id` attribute
  only for a positive Jid. The numeric identifier is safe, but the tag name is
  unescaped application-controlled HTML syntax.

Do not relax canonical `ParseString` parsing to match browser DOM tolerance.
The client and wire routing depend on one stable textual representation.

## Verification

Run `go test -race ./lib/jid` and `go test ./lib/jid` from the module root.
Keep table and fuzz coverage for zero/invalid distinctions, canonical versus
numeric parsing, `int64` bounds, append-prefix preservation, round trips, and
start-tag emission.
