package ui

import (
	"fmt"
	"io"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type jsVarBatchFanoutState struct {
	Value int `json:"value"`
	ack   chan struct{}
}

func (state *jsVarBatchFanoutState) JawsPathSet(*jaws.Element, string, any) {
	state.ack <- struct{}{}
}

func TestJsVarBatchKeepsLatestPartialPaths(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := jaws.New()
		if err != nil {
			t.Fatal(err)
		}
		go jw.Serve()
		tr := jawstest.NewTestRequest(jw, nil)
		<-tr.ReadyCh
		defer func() {
			tr.Cancel(nil)
			tr.Close()
			<-tr.DoneCh
			jw.Close()
			synctest.Wait()
		}()

		var mu sync.Mutex
		state := struct {
			Left  int `json:"left"`
			Right int `json:"right"`
		}{}
		jsvar := NewJsVar(&mu, &state)
		elem := tr.NewElement(jsvar)
		if err = elem.JawsRender(io.Discard, []any{"state"}); err != nil {
			t.Fatal(err)
		}

		for _, write := range []struct {
			path  string
			value int
		}{{"left", 1}, {"right", 2}, {"left", 3}} {
			if err = jsvar.JawsSetPath(elem, write.path, write.value); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
		synctest.Wait()

		var got []string
		for {
			select {
			case msg := <-tr.OutCh:
				if msg.What != what.Set || msg.Jid != elem.Jid() {
					t.Fatalf("JsVar broadcast = %#v, want Set for the bound element", msg)
				}
				got = append(got, msg.Data)
			default:
				if want := []string{"right=2", "left=3"}; !slices.Equal(got, want) {
					t.Fatalf("JsVar path broadcasts = %q, want %q", got, want)
				}
				return
			}
		}
	})
}

func TestJsVarBatchCoalescesAcrossSharedBindings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := jaws.New()
		if err != nil {
			t.Fatal(err)
		}
		go jw.Serve()
		first := jawstest.NewTestRequest(jw, nil)
		second := jawstest.NewTestRequest(jw, nil)
		<-first.ReadyCh
		<-second.ReadyCh
		defer func() {
			first.Cancel(nil)
			second.Cancel(nil)
			first.Close()
			second.Close()
			<-first.DoneCh
			<-second.DoneCh
			jw.Close()
			synctest.Wait()
		}()

		var mu sync.Mutex
		state := struct {
			Value int `json:"value"`
		}{}
		firstJsVar := NewJsVar(&mu, &state)
		secondJsVar := NewJsVar(&mu, &state)
		firstElem := first.NewElement(firstJsVar)
		secondElem := second.NewElement(secondJsVar)
		if err = firstElem.JawsRender(io.Discard, []any{"state"}); err != nil {
			t.Fatal(err)
		}
		if err = secondElem.JawsRender(io.Discard, []any{"state"}); err != nil {
			t.Fatal(err)
		}
		if err = firstJsVar.JawsSetPath(firstElem, "value", 1); err != nil {
			t.Fatal(err)
		}
		if err = secondJsVar.JawsSetPath(secondElem, "value", 2); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
		synctest.Wait()

		for _, tr := range []*jawstest.TestRequest{first, second} {
			select {
			case msg := <-tr.OutCh:
				if msg.What != what.Set || msg.Data != "value=2" {
					t.Fatalf("shared binding broadcast = %#v, want latest value=2 Set", msg)
				}
			default:
				t.Fatal("shared binding did not receive the latest Set")
			}
			select {
			case msg := <-tr.OutCh:
				t.Fatalf("shared binding received extra broadcast %#v", msg)
			default:
			}
		}
	})
}

func TestJsVarClientBurstFanoutKeepsSlowPeerCurrent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := jaws.New()
		if err != nil {
			t.Fatal(err)
		}
		go jw.Serve()
		first := jawstest.NewTestRequest(jw, nil)
		second := jawstest.NewTestRequest(jw, nil)
		<-first.ReadyCh
		<-second.ReadyCh
		firstFrames := make(chan wire.WsMsg, 256)
		firstDrained := make(chan struct{})
		go func() {
			defer close(firstDrained)
			for msg := range first.OutCh {
				firstFrames <- msg
			}
		}()
		defer func() {
			first.Cancel(nil)
			second.Cancel(nil)
			first.Close()
			second.Close()
			<-first.DoneCh
			<-second.DoneCh
			<-firstDrained
			jw.Close()
			synctest.Wait()
		}()

		var mu sync.Mutex
		state := jsVarBatchFanoutState{ack: make(chan struct{}, 1)}
		firstJsVar := NewJsVar(&mu, &state)
		secondJsVar := NewJsVar(&mu, &state)
		checkCalls := 0
		firstJsVar.ClientCheck = func(*jsVarBatchFanoutState, string) error {
			checkCalls++
			return nil
		}
		firstElem := first.NewElement(firstJsVar)
		secondElem := second.NewElement(secondJsVar)
		if err = firstElem.JawsRender(io.Discard, []any{"state"}); err != nil {
			t.Fatal(err)
		}
		if err = secondElem.JawsRender(io.Discard, []any{"state"}); err != nil {
			t.Fatal(err)
		}

		const writes = 160
		for i := 1; i <= writes; i++ {
			first.InCh <- wire.WsMsg{Jid: firstElem.Jid(), What: what.Set, Data: fmt.Sprintf("value=%d", i)}
			<-state.ack
		}
		if checkCalls != writes {
			t.Fatalf("ClientCheck calls = %d, want %d", checkCalls, writes)
		}
		synctest.Wait()
		time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
		synctest.Wait()

		for _, tr := range []*jawstest.TestRequest{first, second} {
			if cause := tr.Context().Err(); cause != nil {
				t.Fatalf("request %v canceled by JsVar burst: %v", tr.JawsKey, cause)
			}
		}
		select {
		case msg := <-second.OutCh:
			if msg.What != what.Set || msg.Data != "value=160" || msg.Jid != secondElem.Jid() {
				t.Fatalf("slow peer broadcast = %#v, want final path Set", msg)
			}
		default:
			t.Fatal("slow peer did not receive the final JsVar path value")
		}
		select {
		case msg := <-second.OutCh:
			t.Fatalf("slow peer received extra broadcast %#v", msg)
		default:
		}
		select {
		case msg := <-firstFrames:
			if msg.What != what.Set || msg.Data != "value=160" || msg.Jid != firstElem.Jid() {
				t.Fatalf("origin broadcast = %#v, want final path Set", msg)
			}
		default:
			t.Fatal("origin did not receive the final JsVar path value")
		}
		select {
		case msg := <-firstFrames:
			t.Fatalf("origin received extra broadcast %#v", msg)
		default:
		}
	})
}
