# AI guidance for github.com/linkdata/jaws/lib/key

See the [module-wide AI guidance](../../AI.md) before changing this package.

## Encoding contract

`Key` is the textual codec used for JaWS request and session keys. Non-zero
values encode as canonical lowercase base-32 text over the full `uint64` range.
`Key(0)` is invalid and encodes as an empty string; the literal `0`, empty input,
and malformed input therefore all parse to the same zero value.

`Append` must append to and preserve its destination slice. `Key.String` is
defined by the same encoding. Keep their outputs identical and allocation-light;
`BenchmarkAppend` is the regression benchmark for this path.

## Parsing and route tails

`Parse` splits at the first ASCII slash. It parses only the prefix and returns
the slash plus all remaining bytes unchanged as `tail`, even when the prefix is
empty or invalid. This supports routes such as `<key>/noscript` without making
the key package aware of route meanings.

Base-32 decoding follows `strconv.ParseUint`: letters are case-insensitive,
while rendering is lowercase. Consequently an uppercase prefix is accepted but
normalizes when re-encoded. Overflow, signs, non-base-32 characters, and other
invalid prefixes return a zero Key while preserving any tail.

## Security boundary

This package only encodes and parses integers. It does not generate randomness,
guarantee uniqueness, bind keys to clients, enforce single-use claims, retire
pending requests, or authorize a route. Those request/session security
properties belong to package `jaws`. Do not add acceptance policy to this codec
or treat successful parsing as proof that a key is live or authorized.

Custom routers should parse the trailing route component here and then ask the
owning `jaws.Jaws` to claim or look up the corresponding object according to
the root package contract.

## Verification

Run `go test -race ./lib/key` and `go test ./lib/key` from the module root.
Keep table and fuzz coverage across the full `uint64` domain, lowercase
canonical rendering, case-insensitive parsing, overflow and invalid input,
slash-tail preservation, destination-prefix preservation, and round trips.
Run `go test ./lib/key -bench=Append -benchmem` for encoding-path changes.
