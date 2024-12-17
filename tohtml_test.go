package jaws

import (
	"reflect"
	"sync"
	"testing"
)

func TestToHTML(t *testing.T) {
	var mu sync.Mutex

	txt := "<p>text</p>"
	htmler := ToHTML(Stringer(&mu, &txt), "<div>%s</div>")
	if s := htmler.JawsGetHTML(nil); s != "<div>&lt;p&gt;text&lt;/p&gt;</div>" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, htmler); !reflect.DeepEqual(tags, []any{&txt}) {
		t.Error(tags)
	}
}
