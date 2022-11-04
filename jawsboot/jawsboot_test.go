package jawsboot_test

import (
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/matryer/is"
)

func TestJawsBoot_Setup(t *testing.T) {
	is := is.New(t)
	jw := jaws.New()
	jawsboot.Setup(jw)
	go jw.Serve()
	defer jw.Close()

	jawsKey := uint64(0xcafebabe)
	txt := jw.HeadHTML(jawsKey)
	is.Equal(strings.Contains(string(txt), jaws.JawsKeyString(jawsKey)), true)
	is.Equal(strings.Contains(string(txt), jaws.JavascriptPath), true)
	is.Equal(strings.Contains(string(txt), jawsboot.DefaultBootstrapVersion), true)
	is.Equal(strings.Contains(string(txt), jawsboot.DefaultBootstrapCDN), true)
}
