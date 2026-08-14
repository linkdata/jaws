// Package templatereloader provides a build-aware
// [github.com/linkdata/jaws.TemplateLookuper].
//
// [New] parses templates from an [io/fs.FS] once in normal builds. Race and debug
// builds instead return a [TemplateReloader] that reparses the corresponding files
// from disk, so fpath and relpath must describe the same logical template set in
// both modes.
//
// Reload failures retain the last successfully parsed templates. Use
// [TemplateReloader.LastError] and [TemplateReloader.Path] for diagnostics.
package templatereloader
