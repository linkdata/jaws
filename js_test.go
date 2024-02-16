package jaws

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"hash/fnv"
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
	th.True(len(JavascriptGZip) > 0)
	th.True(len(JavascriptGZip) < len(JavascriptText))

	b := bytes.Buffer{}
	gw := gzip.NewWriter(&b)
	_, err = gw.Write(JavascriptText)
	th.NoErr(err)
	th.NoErr(gw.Close())
	th.Equal(b.Bytes(), JavascriptGZip)
}

func Test_HeadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js"
	const extraStyle = "someExtraStyle.css"
	th := newTestHelper(t)
	txt := HeadHTML(nil, nil)
	th.Equal(strings.Contains(string(txt), JavascriptPath), false)
	txt = HeadHTML([]string{JavascriptPath, extraScript}, []string{extraStyle})
	th.Equal(strings.Contains(string(txt), JavascriptPath), true)
	th.Equal(strings.Contains(string(txt), extraScript), true)
	th.Equal(strings.Contains(string(txt), extraStyle), true)
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
