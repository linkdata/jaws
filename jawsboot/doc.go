// Package jawsboot provides embedded Bootstrap assets for JaWS applications.
//
// The embedded assets are Bootstrap v5.3.8, downloaded from
// https://getbootstrap.com/ (bootstrap.bundle.min.js and bootstrap.min.css,
// stored gzip-compressed under assets/static). When bumping the vendored files,
// update this version note and the comment at the //go:embed directive so the
// shipped release stays auditable against upstream security advisories.
package jawsboot
