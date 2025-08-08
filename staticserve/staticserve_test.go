package staticserve_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

const someJavascript = `var jaws = null;

function jawsContains(a, v) {
	return a.indexOf(String(v).trim().toLowerCase()) !== -1;
}
`

func Test_New(t *testing.T) {
	ss, err := staticserve.New("test.js", []byte(someJavascript))
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(ss.ContentType, "javascript") {
		t.Error("ss not javascript")
	}
	ss2, err := staticserve.New("test.js.gz", ss.Gz)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(ss2.Gz, ss.Gz) {
		t.Error("bytes differ")
	}
	if !strings.Contains(ss2.ContentType, "javascript") {
		t.Error("ss2 not javascript")
	}
	if ss.Name != ss2.Name {
		t.Error(ss.Name, "!=", ss2.Name)
	}
	ss3, err := staticserve.New("test.foo123", nil)
	if err != nil {
		t.Error(err)
	}
	if ss3.ContentType != "" {
		t.Error(ss3.ContentType)
	}
}

func Test_Must(t *testing.T) {
	ss := staticserve.Must("test", nil)
	if ss == nil {
		t.FailNow()
	}
}

func Test_MaybePanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fail()
		}
	}()
	staticserve.MaybePanic(io.EOF)
	t.Fail()
}
