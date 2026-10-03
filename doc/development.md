# Development

[Documentation](README.md) · [Testing](testing.md) · [Transport](transport.md)

Run repository commands from the module root using the Go version in
[go.mod](../go.mod) or newer. Browser-client tests use Node.js.

## Run tests

```sh
JAWS_REQUIRE_NODE=1 go test ./...
JAWS_REQUIRE_NODE=1 go test -race ./...
```

`JAWS_REQUIRE_NODE=1` fails when Node is missing instead of skipping browser
behavior tests. The race build enables debug checks, deadlock detection, and
the detailed tag renderer. The ordinary build covers production behavior.
On platforms without the race detector, the debug checks can run separately:

```sh
JAWS_REQUIRE_NODE=1 go test -tags "debug deadlock" ./...
```

Run a package in isolation by replacing `./...`, for example with `./lib/bind`.
[Testing](testing.md) describes the test harness and runnable examples.
The `build-386` CI job covers 32-bit numeric bounds; it requires a host capable
of executing 32-bit binaries.

## Generate and check

The [build workflow](../.github/workflows/build.yml) records CI tool configuration
and checks, including:

```sh
go generate ./...
go vet ./...
gofmt -l .
staticcheck ./...
golangci-lint run
gosec ./...
go build ./...
```

Generation updates command names in `lib/what`; browser assets are embedded
directly from their source files. The
[Bootstrap guide](bootstrap.md#asset-provenance-and-updates) covers updating the
bundled third-party files. For development-time HTML editing, use
[template reloading](utilities.md#reload-templates-during-development).

## Measure performance

Benchmarks live beside their packages. For example:

```sh
go test -run '^$' -bench '^BenchmarkContainerUnchangedUpdate$' -benchmem -count 6 ./lib/ui
```

Collect before and after runs under matching conditions and compare them with
`benchstat`. `b.RunParallel` exercises contention; `b.ReportAllocs` exposes
per-operation allocation costs.
