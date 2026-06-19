package htmlio_test

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/jid"
)

func ExampleWriteHTMLInner_escapedText() {
	userText := `<b onclick="alert(1)">Ada & Bob</b>`
	safeInner := template.HTML(template.HTMLEscapeString(userText)) // #nosec G203

	var sb strings.Builder
	if err := htmlio.WriteHTMLInner(&sb, jid.Jid(7), "span", "", safeInner, htmlio.Attr("title", userText)); err != nil {
		panic(err)
	}
	fmt.Println(sb.String())

	// Output: <span id="Jid.7" title="&lt;b onclick=&#34;alert(1)&#34;&gt;Ada &amp; Bob&lt;/b&gt;">&lt;b onclick=&#34;alert(1)&#34;&gt;Ada &amp; Bob&lt;/b&gt;</span>
}
