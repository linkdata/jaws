# AI guidance for github.com/linkdata/jaws/lib/htmlio

See the [module-wide AI guidance](../../AI.md) before changing this package.

## Trust and escaping model

This package is the low-level HTML writer used by standard JaWS widgets. HTML
tag names, attribute names, `template.HTMLAttr` fragments, and
`template.HTML` inner content are trusted inputs written verbatim. They are not
validated or made safe by browser parser recovery. Never derive those inputs
from untrusted data.

Attribute values are the escaping boundary. `AppendAttrValue` accepts an
unescaped logical value and emits a double-quoted HTML attribute value using
the template escaper. It preserves carriage returns through `&#13;` because
HTML input preprocessing would otherwise normalize them, and U+0000 becomes
U+FFFD. `AppendAttr` and `Attr` combine that encoding with a trusted attribute
name. `AppendAttrs` only joins already-trusted raw fragments.

## Element emission

`WriteHTMLTag` writes a start tag, optional positive Jid, optional escaped type
and value attributes, and trusted attribute fragments. `WriteHTMLInput` is its
input-specific wrapper. Jid emission belongs to `lib/jid`: zero and negative
identifiers do not produce an `id` attribute.

`WriteHTMLInner` writes trusted inner HTML and a closing tag for non-void
elements. Its behavior is intentionally HTML-aware:

- Void elements have no closing tag and ignore supplied inner HTML.
- `textarea` and `pre` receive one source LF after the opening tag so the HTML
  parser consumes that prefix rather than a leading LF from the intended
  content.
- It does not synthesize a `value` attribute; callers pass one explicitly via
  `Attr` when needed.
- Carriage returns in inner HTML remain verbatim. Browser textarea values may
  normalize them; only attribute-value emission provides CR preservation.

Keep tag-name handling case-insensitive only where HTML semantics require it
(void and newline-sensitive classification). Preserve the caller's original
tag spelling in emitted markup and closing tags.

## Widget-authoring rules

Prefer these helpers over manual string concatenation when implementing a
low-level widget. Build dynamic attribute values with `Attr`/`AppendAttr`, pass
only application-controlled names and raw fragments, and propagate the Writer
error without replacing it with an unrelated error. HTML text that is not
already trusted must be escaped before conversion to `template.HTML`.

## Verification

Run `go test -race ./lib/htmlio` and `go test ./lib/htmlio` from the module root.
Keep table and fuzz coverage for escaping, CR/NUL behavior, DOM-parsed values,
void elements, textarea/pre leading newlines, positive/zero/invalid Jids, and
Writer-error propagation.
