#!/usr/bin/env bash
set -euo pipefail

if (( $# < 1 || $# > 2 )); then
	echo "usage: $0 {before|after} [result-directory]" >&2
	exit 2
fi
if [[ $1 != before && $1 != after ]]; then
	echo "usage: $0 {before|after} [result-directory]" >&2
	exit 2
fi

label=$1
result_dir=${2:-${TMPDIR:-/tmp}/jaws-remove-bench}
root=$(git rev-parse --show-toplevel)
cd "$root"

command -v benchstat >/dev/null || {
	echo "benchstat is required" >&2
	exit 1
}

mkdir -p "$result_dir"
raw=$result_dir/$label.txt
meta=$result_dir/$label.meta
tests='^(TestRequest_DeleteElementNil|TestRequest_DeleteElements|TestRequest_IncomingRemove.*)$'

GOTOOLCHAIN=local GOPROXY=off go test -race -count=1 -run "$tests" .
GOTOOLCHAIN=local GOPROXY=off go test -count=1 -run "$tests" .

{
	echo "label=$label"
	echo "commit=$(git rev-parse HEAD)"
	go version
	git status --short
} >"$meta"

GOTOOLCHAIN=local GOPROXY=off go test -run '^$' \
	-bench '^BenchmarkRequestIncomingRemoveCleanup$' \
	-benchmem -benchtime=3x -count=10 -cpu=1 . | tee "$raw"

before=$result_dir/before.txt
after=$result_dir/after.txt
if [[ -f $before && -f $after ]]; then
	benchstat "$before" "$after" | tee "$result_dir/comparison.txt"
else
	echo "saved $raw; run $0 after '$result_dir' after applying the fix"
fi
