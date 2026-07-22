package htmlio_test

import (
	"html"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/jid"
)

func Test_WriteHTMLInner(t *testing.T) {
	type args struct {
		jid   jid.Jid
		tag   string
		typ   string
		inner template.HTML
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLInner no attrs",
			args: args{
				jid:   1,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
			},
			want: `<tag1 id="Jid.1" type="typ1">inner_text</tag1>`,
		},
		{
			name: "HTMLInner singleton tag",
			args: args{
				jid:   2,
				tag:   "img",
				typ:   "",
				inner: "",
			},
			want: `<img id="Jid.2">`,
		},
		{
			name: "HTMLInner two filled attrs, one empty",
			args: args{
				jid:   3,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
				attrs: []template.HTMLAttr{"some_attr1", "some_attr2", ""},
			},
			want: `<tag1 id="Jid.3" type="typ1" some_attr1 some_attr2>inner_text</tag1>`,
		},
		{
			name: "HTMLInner void tag drops inner content",
			args: args{
				jid:   4,
				tag:   "img",
				inner: "ignored",
			},
			want: `<img id="Jid.4">`,
		},
		{
			name: "HTMLInner uppercase void tag drops inner content",
			args: args{
				jid:   5,
				tag:   "IMG",
				inner: "ignored",
			},
			want: `<IMG id="Jid.5">`,
		},
		{
			name: "HTMLInner non-positive jid omits id attribute",
			args: args{
				jid:   0,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
			},
			want: `<tag1 type="typ1">inner_text</tag1>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := htmlio.WriteHTMLInner(&sb, tt.args.jid, tt.args.tag, tt.args.typ, tt.args.inner, tt.args.attrs...); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLInner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_WriteHTMLInner_ClosingTag(t *testing.T) {
	// Void/singleton tags must never get a closing tag (and their inner
	// content is dropped); ordinary tags must always be closed.
	voidTags := []string{
		"area", "base", "br", "col", "embed", "hr", "img", "input",
		"link", "meta", "param", "source", "track", "wbr",
	}
	nonVoidTags := []string{"div", "span", "section", "p"}

	for _, tag := range voidTags {
		t.Run("void/"+tag, func(t *testing.T) {
			var sb strings.Builder
			if err := htmlio.WriteHTMLInner(&sb, jid.Jid(1), tag, "", "inner"); err != nil {
				t.Fatal(err)
			}
			if got, closing := sb.String(), "</"+tag+">"; strings.Contains(got, closing) {
				t.Errorf("WriteHTMLInner(%q) = %q, must not contain closing tag %q", tag, got, closing)
			}
		})
	}

	for _, tag := range nonVoidTags {
		t.Run("nonvoid/"+tag, func(t *testing.T) {
			var sb strings.Builder
			if err := htmlio.WriteHTMLInner(&sb, jid.Jid(1), tag, "", "inner"); err != nil {
				t.Fatal(err)
			}
			if got, closing := sb.String(), "</"+tag+">"; !strings.Contains(got, closing) {
				t.Errorf("WriteHTMLInner(%q) = %q, must contain closing tag %q", tag, got, closing)
			}
		})
	}
}

func Test_WriteHTMLInner_NewlineSensitivePrefix(t *testing.T) {
	tests := []struct {
		name  string
		tag   string
		inner template.HTML
		want  string
	}{
		{
			name:  "textarea leading LF is preserved after prefix",
			tag:   "textarea",
			inner: "\nhello",
			want:  "<textarea id=\"Jid.1\">\n\nhello</textarea>",
		},
		{
			// A carriage return is written verbatim: the textarea value API
			// normalizes it to LF (see TestWriteHTMLInner_TextareaValueNormalization),
			// so encoding it would not round-trip anyway.
			name:  "textarea carriage returns are written verbatim",
			tag:   "textarea",
			inner: "\ra\r\nb",
			want:  "<textarea id=\"Jid.1\">\n\ra\r\nb</textarea>",
		},
		{
			// pre content is trusted markup written verbatim; encoding CR would
			// corrupt nested script/comment contexts that do not decode &#13;.
			name:  "pre carriage returns are written verbatim",
			tag:   "pre",
			inner: "<script>//x\rdoThing()</script>",
			want:  "<pre id=\"Jid.1\">\n<script>//x\rdoThing()</script></pre>",
		},
		{
			name:  "uppercase TEXTAREA is newline-sensitive",
			tag:   "TEXTAREA",
			inner: "\nhello",
			want:  "<TEXTAREA id=\"Jid.1\">\n\nhello</TEXTAREA>",
		},
		{
			name:  "pre leading LF is preserved after prefix",
			tag:   "pre",
			inner: "\nhello",
			want:  "<pre id=\"Jid.1\">\n\nhello</pre>",
		},
		{
			name:  "textarea ordinary content gets parser prefix",
			tag:   "textarea",
			inner: "hello",
			want:  "<textarea id=\"Jid.1\">\nhello</textarea>",
		},
		{
			name: "empty textarea gets parser prefix",
			tag:  "textarea",
			want: "<textarea id=\"Jid.1\">\n</textarea>",
		},
		{
			name:  "div leading newline is unchanged",
			tag:   "div",
			inner: "\nx",
			want:  "<div id=\"Jid.1\">\nx</div>",
		},
		{
			// WriteHTMLInner never rewrites innerHTML; the trusted markup,
			// including any carriage return, is carried through verbatim.
			name:  "div carriage return is left verbatim",
			tag:   "div",
			inner: "a\rb",
			want:  "<div id=\"Jid.1\">a\rb</div>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := htmlio.WriteHTMLInner(&sb, jid.Jid(1), tt.tag, "", tt.inner); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("WriteHTMLInner() = %q, want %q", got, tt.want)
			}
		})
	}
}

// FuzzAppendAttrValue guards the escaping invariant: for arbitrary input, the
// bytes between the bounding double quotes must contain no raw '"' or '<' that
// could break out of the attribute value or open a new tag.
func FuzzAppendAttrValue(f *testing.F) {
	for _, seed := range []string{"", `"`, "<", `"&<>'`, "plain", "<script>"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got := string(htmlio.AppendAttrValue(nil, value))
		if len(got) < 2 || got[0] != '"' || got[len(got)-1] != '"' {
			t.Fatalf("AppendAttrValue(%q) = %q, want double-quoted", value, got)
		}
		if inner := got[1 : len(got)-1]; strings.ContainsAny(inner, "\"<") {
			t.Fatalf("AppendAttrValue(%q) inner value %q contains raw '\"' or '<'", value, inner)
		}
	})
}

func Test_WriteHTMLInput(t *testing.T) {
	type args struct {
		jid   jid.Jid
		typ   string
		val   string
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLInput no attrs",
			args: args{
				jid: 1,
				typ: "input_type",
				val: "initial_val",
			},
			want: `<input id="Jid.1" type="input_type" value="initial_val">`,
		},
		{
			name: "HTMLInput one empty attr",
			args: args{
				jid:   2,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{""},
			},
			want: `<input id="Jid.2" type="input_type2" value="initial_val2">`,
		},
		{
			name: "HTMLInput one filled attr",
			args: args{
				jid:   3,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{"some_attr"},
			},
			want: `<input id="Jid.3" type="input_type2" value="initial_val2" some_attr>`,
		},
		{
			name: "HTMLInput two filled attr, one empty",
			args: args{
				jid:   4,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{"some_attr1", "", "some_attr2"},
			},
			want: `<input id="Jid.4" type="input_type2" value="initial_val2" some_attr1 some_attr2>`,
		},
		{
			name: "HTMLInput escapes generated attr values",
			args: args{
				jid: 5,
				typ: `"&<>'\` + "\n",
				val: `"&<>'\` + "\n",
			},
			want: "<input id=\"Jid.5\" type=\"&#34;&amp;&lt;&gt;&#39;\\\n\" value=\"&#34;&amp;&lt;&gt;&#39;\\\n\">",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := htmlio.WriteHTMLInput(&sb, tt.args.jid, tt.args.typ, tt.args.val, tt.args.attrs); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendAttr(t *testing.T) {
	value := `"&<>'\` + "\n"
	got := string(htmlio.AppendAttr(nil, "data-x", value))
	want := " data-x=\"&#34;&amp;&lt;&gt;&#39;\\\n\""
	if got != want {
		t.Fatalf("AppendAttr() = %q, want %q", got, want)
	}
	if strings.Contains(got, `\"`) || strings.Contains(got, `\n`) {
		t.Fatalf("AppendAttr() used Go/JavaScript-style escapes: %q", got)
	}
}

func TestAttr(t *testing.T) {
	value := `"&<>'\` + "\n"
	var attr template.HTMLAttr = htmlio.Attr("data-x", value)
	got := string(attr)
	want := "data-x=\"&#34;&amp;&lt;&gt;&#39;\\\n\""
	if got != want {
		t.Fatalf("Attr() = %q, want %q", got, want)
	}
	if strings.HasPrefix(got, " ") {
		t.Fatalf("Attr() returned a leading space: %q", got)
	}
	if strings.Contains(got, `\"`) || strings.Contains(got, `\n`) {
		t.Fatalf("Attr() used Go/JavaScript-style escapes: %q", got)
	}
}

func TestWriteHTMLTag(t *testing.T) {
	var sb strings.Builder
	attrs := []template.HTMLAttr{"some_attr", "", "other"}
	if err := htmlio.WriteHTMLTag(&sb, jid.Jid(7), "div", "typ", "val", attrs); err != nil {
		t.Fatal(err)
	}
	// htmlTag and attrs are written verbatim; type/value are escaped.
	want := `<div id="Jid.7" type="typ" value="val" some_attr other>`
	if got := sb.String(); got != want {
		t.Fatalf("WriteHTMLTag() = %q, want %q", got, want)
	}
}

func TestAppendAttrValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "html metacharacters",
			value: `"&<>'`,
			want:  `"&#34;&amp;&lt;&gt;&#39;"`,
		},
		{
			// html.EscapeString leaves CR raw, but browser preprocessing would
			// rewrite it to LF; it must be a numeric character reference instead.
			name:  "carriage return is encoded",
			value: "a\rb",
			want:  `"a&#13;b"`,
		},
		{
			name:  "CRLF encodes only the CR",
			value: "a\r\nb",
			want:  "\"a&#13;\nb\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(htmlio.AppendAttrValue(nil, tt.value)); got != tt.want {
				t.Fatalf("AppendAttrValue(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestAppendAttrs(t *testing.T) {
	got := string(htmlio.AppendAttrs(nil, []template.HTMLAttr{"x", "", "y"}))
	if want := " x y"; got != want {
		t.Fatalf("AppendAttrs() = %q, want %q (empty fragments must be skipped)", got, want)
	}
	if got := string(htmlio.AppendAttrs(nil, nil)); got != "" {
		t.Fatalf("AppendAttrs(nil) = %q, want empty", got)
	}
}

// errWriter is an io.Writer that always fails, to verify error propagation.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

func TestWriteHTML_WriterErrorPropagates(t *testing.T) {
	if err := htmlio.WriteHTMLTag(errWriter{}, jid.Jid(1), "div", "", "", nil); err != io.ErrShortWrite {
		t.Errorf("WriteHTMLTag error = %v, want %v", err, io.ErrShortWrite)
	}
	if err := htmlio.WriteHTMLInput(errWriter{}, jid.Jid(1), "text", "v", nil); err != io.ErrShortWrite {
		t.Errorf("WriteHTMLInput error = %v, want %v", err, io.ErrShortWrite)
	}
	if err := htmlio.WriteHTMLInner(errWriter{}, jid.Jid(1), "span", "", "x"); err != io.ErrShortWrite {
		t.Errorf("WriteHTMLInner error = %v, want %v", err, io.ErrShortWrite)
	}
}

// normalizeCR models the browser HTML input-stream preprocessing step that
// rewrites every CRLF pair and every standalone CR to a single LF before
// tokenization. This is why a raw carriage return never survives HTML parsing.
func normalizeCR(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// TestAppendAttrValue_DOMRoundTrip verifies that a logical attribute value with
// carriage returns survives a browser parse unchanged, as getAttribute reports
// it. It models the two browser steps that would otherwise corrupt a raw CR:
// input-stream preprocessing (CR/CRLF to LF) followed by tokenizer
// character-reference decoding, the latter using the standard library
// html.UnescapeString.
func TestAppendAttrValue_DOMRoundTrip(t *testing.T) {
	values := []string{"", "plain", "a\rb", "a\r\nb", "\rlead", "trail\r", "x\ry\rz", `"&<>'` + "\r"}
	for _, value := range values {
		src := string(htmlio.AppendAttrValue(nil, value))
		inner := src[1 : len(src)-1] // strip the bounding double quotes
		if got := html.UnescapeString(normalizeCR(inner)); got != value {
			t.Errorf("attribute value %q round-tripped to %q via source %q", value, got, src)
		}
	}
}

// innerContent returns the HTML source between an element's start tag and its
// matching end tag.
func innerContent(t *testing.T, source, tag string) string {
	t.Helper()
	start := strings.IndexByte(source, '>')
	end := strings.LastIndex(source, "</"+tag+">")
	if start < 0 || end < start {
		t.Fatalf("cannot locate <%s> content in %q", tag, source)
	}
	return source[start+1 : end]
}

// TestWriteHTMLInner_TextareaValueNormalization documents that a textarea cannot
// round-trip carriage returns: the value the browser reports (and jaws.js sends
// back) collapses every CR and CRLF to LF. It mirrors the ui.Textarea pipeline
// (the value is HTML-escaped before WriteHTMLInner) and models
// HTMLTextAreaElement.value: input-stream preprocessing, character-reference
// decoding, dropping the single leading LF the parser strips after the start
// tag to obtain the raw value, then the textarea value normalization the HTML
// standard applies (CR and CRLF become LF).
func TestWriteHTMLInner_TextareaValueNormalization(t *testing.T) {
	values := []string{"", "plain", "\rhello", "\nhello", "a\rb", "a\r\nb\rc", "\r\r", "<kept>&amp;"}
	for _, value := range values {
		inner := template.HTML(template.HTMLEscapeString(value)) // #nosec G203
		var sb strings.Builder
		if err := htmlio.WriteHTMLInner(&sb, jid.Jid(1), "textarea", "", inner); err != nil {
			t.Fatal(err)
		}
		content := innerContent(t, sb.String(), "textarea")
		rawValue := strings.TrimPrefix(html.UnescapeString(normalizeCR(content)), "\n")
		apiValue := normalizeCR(rawValue) // textarea value normalization: CR/CRLF -> LF
		if want := normalizeCR(value); apiValue != want {
			t.Errorf("textarea value %q reported as %q, want %q (source %q)", value, apiValue, want, sb.String())
		}
	}
}
