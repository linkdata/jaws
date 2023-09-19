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
	defer jw.Close()
	is.NoErr(jawsboot.Setup(jw))

	rq := jw.NewRequest(nil)
	txt := rq.HeadHTML()
	is.Equal(strings.Contains(string(txt), rq.JawsKeyString()), true)
	is.Equal(strings.Contains(string(txt), jaws.JavascriptPath), true)
	is.Equal(strings.Contains(string(txt), jawsboot.DefaultBootstrapVersion), true)
	is.Equal(strings.Contains(string(txt), jawsboot.DefaultBootstrapCDN), true)
}
