// Package ui contains the standard JaWS widget implementations.
//
// The package is intentionally organized around extension-oriented building
// blocks so new widgets can be authored here without reading JaWS core code:
//
//   - `HTMLInner`: base renderer for tags with inner HTML content.
//   - `Input`, `InputText`, `InputBool`, `InputFloat`, `InputDate`:
//     typed input helpers that handle event/update flow.
//   - `WrapContainer`: helper for widgets that render dynamic child UI lists.
//
// Naming follows short widget names (`Span`, `NewSpan`) instead of the
// legacy core names (`UiSpan`, `NewUiSpan`).
package ui
