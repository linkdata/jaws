package jaws

import (
	"reflect"
	"sync"
	"testing"
)

func TestHTMLer(t *testing.T) {
	var mu sync.Mutex
	var mu2 sync.RWMutex

	txt := "<text>"
	htmler := HTMLer(&mu, "<p>%s</p>", Fmt(&txt))
	if s := htmler.JawsGetHTML(nil); s != "<p>&lt;text&gt;</p>" {
		t.Error(s)
	}
	tags, err := TagExpand(nil, htmler)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(tags, []any{&txt}) {
		t.Error(tags)
	}

	num := 123
	htmler = HTMLer(&mu2, "<p>%s%s%s</p>", Fmt(&txt), "<!>", Fmt(&num))
	if s := htmler.JawsGetHTML(nil); s != "<p>&lt;text&gt;&lt;!&gt;123</p>" {
		t.Error(s)
	}
	tags, err = TagExpand(nil, htmler)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(tags, []any{&txt, &num}) {
		t.Error(tags)
	}
}
