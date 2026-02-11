package core

import (
	"html/template"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestCoverage_GenerateHeadAndConvenienceBroadcasts(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err := jw.GenerateHeadHTML("%zz"); err == nil {
		t.Fatal("expected url parse error")
	}
	if err := jw.GenerateHeadHTML("/favicon.ico", "/app.js"); err != nil {
		t.Fatal(err)
	}

	jw.Reload()
	if msg := nextBroadcast(t, jw); msg.What != what.Reload {
		t.Fatalf("unexpected reload msg %#v", msg)
	}
	jw.Redirect("/next")
	if msg := nextBroadcast(t, jw); msg.What != what.Redirect || msg.Data != "/next" {
		t.Fatalf("unexpected redirect msg %#v", msg)
	}
	jw.Alert("info", "hello")
	if msg := nextBroadcast(t, jw); msg.What != what.Alert || msg.Data != "info\nhello" {
		t.Fatalf("unexpected alert msg %#v", msg)
	}

	jw.SetInner("t", template.HTML("<b>x</b>"))
	if msg := nextBroadcast(t, jw); msg.What != what.Inner || msg.Data != "<b>x</b>" {
		t.Fatalf("unexpected set inner msg %#v", msg)
	}
	jw.SetAttr("t", "k", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.SAttr || msg.Data != "k\nv" {
		t.Fatalf("unexpected set attr msg %#v", msg)
	}
	jw.RemoveAttr("t", "k")
	if msg := nextBroadcast(t, jw); msg.What != what.RAttr || msg.Data != "k" {
		t.Fatalf("unexpected remove attr msg %#v", msg)
	}
	jw.SetClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.SClass || msg.Data != "c" {
		t.Fatalf("unexpected set class msg %#v", msg)
	}
	jw.RemoveClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.RClass || msg.Data != "c" {
		t.Fatalf("unexpected remove class msg %#v", msg)
	}
	jw.SetValue("t", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected set value msg %#v", msg)
	}
	jw.Insert("t", "0", "<i>a</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Insert || msg.Data != "0\n<i>a</i>" {
		t.Fatalf("unexpected insert msg %#v", msg)
	}
	jw.Replace("t", "0", "<i>b</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Replace || msg.Data != "0\n<i>b</i>" {
		t.Fatalf("unexpected replace msg %#v", msg)
	}
	jw.Delete("t")
	if msg := nextBroadcast(t, jw); msg.What != what.Delete {
		t.Fatalf("unexpected delete msg %#v", msg)
	}
	jw.Append("t", "<em>c</em>")
	if msg := nextBroadcast(t, jw); msg.What != what.Append || msg.Data != "<em>c</em>" {
		t.Fatalf("unexpected append msg %#v", msg)
	}
	jw.JsCall("t", "fn", `{"a":1}`)
	if msg := nextBroadcast(t, jw); msg.What != what.Call || msg.Data != `fn={"a":1}` {
		t.Fatalf("unexpected jscall msg %#v", msg)
	}
}
