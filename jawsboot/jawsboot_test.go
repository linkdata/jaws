package jawsboot_test

import (
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
)

func TestJawsBoot_Setup(t *testing.T) {
	jw := jaws.New()
	defer jw.Close()
	if err := jawsboot.Setup(jw); err != nil {
		t.Fatal(err)
	}

	rq := jw.NewRequest(nil)
	txt := string(rq.HeadHTML())
	if !strings.Contains(txt, rq.JawsKeyString()) {
		t.Error(txt)
	}
	if !strings.Contains(txt, jaws.JavascriptPath) {
		t.Error(txt)
	}
	if !strings.Contains(txt, jawsboot.DefaultBootstrapVersion) {
		t.Error(txt)
	}
	if !strings.Contains(txt, jawsboot.DefaultBootstrapCDN) {
		t.Error(txt)
	}
}
