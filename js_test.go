package jaws

import (
	_ "embed"
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func Test_Javascript(t *testing.T) {
	const prefix = "/jaws/.jaws."
	const suffix = ".js"
	th := newTestHelper(t)

	h := fnv.New64a()
	_, err := h.Write(JavascriptText)
	th.NoErr(err)
	th.Equal(JavascriptPath, prefix+strconv.FormatUint(h.Sum64(), 36)+suffix)
	th.True(len(JavascriptText) > 0)
}

func Test_PreloadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	const extraImage = "favicon.png"
	const extraFont = "someExtraFont.woff2"
	th := newTestHelper(t)

	txt := PreloadHTML()
	th.Equal(strings.Contains(txt, JavascriptPath), false)
	th.Equal(strings.Count(txt, "<script>"), strings.Count(txt, "</script>"))

	mustParseUrl := func(urlstr string) *url.URL {
		u, err := url.Parse(urlstr)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	txt = PreloadHTML(
		mustParseUrl(JavascriptPath),
		mustParseUrl(extraScript),
		mustParseUrl(extraStyle),
		mustParseUrl(extraImage),
		mustParseUrl(extraFont))
	th.Equal(strings.Contains(txt, JavascriptPath), true)
	th.Equal(strings.Contains(txt, extraScript), true)
	th.Equal(strings.Contains(txt, extraStyle), true)
	th.Equal(strings.Contains(txt, extraImage), true)
	th.Equal(strings.Contains(txt, extraFont), true)
	th.Equal(strings.Count(txt, "<script"), strings.Count(txt, "</script>"))
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
