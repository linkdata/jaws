package jawsboot_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
)

func TestJawsBoot_Setup(t *testing.T) {
	oldmux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	jw, _ := jaws.New()
	defer func() {
		jw.Close()
		http.DefaultServeMux = oldmux
	}()
	if err := jawsboot.Setup(jw, nil, "/other/foobar.js"); err != nil {
		t.Fatal(err)
	}

	rq := jw.NewRequest(nil)
	var sb strings.Builder
	rq.Writer(&sb).HeadHTML()
	txt := sb.String()
	if !strings.Contains(txt, rq.JawsKeyString()) {
		t.Error(txt)
	}
	if !strings.Contains(txt, "/static/bootstrap.bundle.min") {
		t.Error(txt)
	}
	if !strings.Contains(txt, "/static/bootstrap.min") {
		t.Error(txt)
	}
	if !strings.Contains(txt, "/other/foobar.js") {
		t.Error(txt)
	}
}
