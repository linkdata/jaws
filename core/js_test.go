package core

import (
	_ "embed"
	"net/url"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

func Test_PreloadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	const extraImage = "favicon.png"
	const extraFont = "someExtraFont.woff2"
	th := newTestHelper(t)

	serveJS, err := staticserve.New("/jaws/.jaws.js", JavascriptText)
	th.NoErr(err)

	txt, fav := PreloadHTML()
	th.Equal(strings.Contains(txt, serveJS.Name), false)
	th.Equal(strings.Count(txt, "<script>"), strings.Count(txt, "</script>"))
	th.Equal(fav, "")

	mustParseUrl := func(urlstr string) *url.URL {
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	txt, fav = PreloadHTML(
		mustParseUrl(serveJS.Name),
		mustParseUrl(extraScript),
		mustParseUrl(extraStyle),
		mustParseUrl(extraImage),
		mustParseUrl(extraFont))
	th.Equal(strings.Contains(txt, serveJS.Name), true)
	th.Equal(strings.Contains(txt, extraScript), true)
	th.Equal(strings.Contains(txt, extraStyle), true)
	th.Equal(strings.Contains(txt, extraImage), true)
	th.Equal(strings.Contains(txt, extraFont), true)
	th.Equal(strings.Count(txt, "<script"), strings.Count(txt, "</script>"))
	th.Equal(fav, extraImage)
	t.Log(txt)
}

func TestJawsKeyString(t *testing.T) {
	th := newTestHelper(t)
	th.Equal(JawsKeyString(0), "")
	th.Equal(JawsKeyString(1), "1")
}

func TestJawsKeyValue(t *testing.T) {
	tests := []struct {
		name    string
		jawsKey string
		want    uint64
	}{
		{
			name:    "blank",
			jawsKey: "",
			want:    0,
		},
		{
			name:    "1",
			jawsKey: "1",
			want:    1,
		},
		{
			name:    "-1",
			jawsKey: "-1",
			want:    0,
		},
		{
			name:    "2/",
			jawsKey: "2/",
			want:    2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JawsKeyValue(tt.jawsKey); got != tt.want {
				t.Errorf("JawsKeyValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
