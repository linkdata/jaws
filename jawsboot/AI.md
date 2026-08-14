# AI guidance for github.com/linkdata/jaws/jawsboot

See the [module-wide AI guidance](../AI.md) before changing this package.

## Package role

`jawsboot` vendors Bootstrap v5.3.8 as the gzip-compressed upstream artifacts
`bootstrap.bundle.min.js` and `bootstrap.min.css` under `assets/static`. The
human-facing provenance table and integration example remain in
[README.md](./README.md).

`Setup` is intended for `jaws.Jaws.Setup`. It walks the embedded assets, exposes
their content-hashed `staticserve` names, and returns the same rooted URL paths
that it registers. Absolute, relative, parent-relative, and empty prefixes are
cleaned through the same path construction. Handler patterns use serialized
URLs so braces and other `http.ServeMux` pattern syntax in a logical prefix are
treated as literal path data.

The predictable un-hashed Bootstrap sourcemap paths are registered with exact
404 handlers. Sourcemaps are not bundled, and devtools probes must not fall
through to an application wildcard route.

## Embedded asset layout

- Keep the two upstream artifacts gzip-compressed directly under
  `assets/static`; `//go:embed assets/static` and `staticserve.WalkDir` depend on
  that tree.
- Keep the JavaScript bundle variant: Bootstrap components used by JaWS alerts
  require the bundled runtime, not only the core Bootstrap script.
- Do not add generated hash manifests or tests that pin repository-tracked blob
  hashes. Git history records the blobs; tests should exercise serving and
  integration behavior.

## Bootstrap version update checklist

1. Obtain the new minified bundle JavaScript and minified CSS artifacts from
   the official Bootstrap distribution and replace their gzip-compressed files
   without renaming them.
2. Update the version and provenance in `README.md`, the public version in
   `doc.go`, this guide, and the `assetsFS` source comment in `jawsboot.go` in
   the same change.
3. Confirm decompression yields the intended upstream filenames/content and
   that both plain and gzip HTTP responses remain valid.
4. Review the sourcemap names registered by `Setup`; update the exact 404 list
   if upstream artifact names change, without bundling maps implicitly.
5. Run the package tests and inspect generated JaWS head markup to ensure both
   returned asset URLs resolve to their registered handlers for every supported
   prefix form.

## Verification

Run `go test -race ./jawsboot` and `go test ./jawsboot` from the module root.
The tests cover plain/gzip serving headers and bodies, URL/handler parity,
literal brace prefixes, nil registration through `Jaws.Setup`, and sourcemap
404 behavior. Also compile-check the README-shaped package example.
