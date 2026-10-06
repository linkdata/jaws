package jaws

import (
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestRequestQueuedWorkWakesIdleLoop(t *testing.T) {
	for _, source := range []string{"command", "event", "dirty", "reload", "maintenance"} {
		t.Run(source, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rq := newTestRequest(t)
				defer closeRequestInBubble(rq)
				rq.Jaws.updateTicker.Stop()
				release := make(chan struct{})
				ui := &testUi{
					renderFn: func(elem *Element, _ io.Writer, _ []any) error {
						elem.AddHandlers(InputFn(func(elem *Element, value string) error {
							<-release
							elem.SetValue(value)
							return nil
						}))
						return nil
					},
					updateFn: func(elem *Element) { elem.SetValue("queued") },
				}
				elem := rq.NewElement(ui)
				if err := elem.JawsRender(io.Discard, nil); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					synctest.Wait()
					want := wire.WsMsg{Jid: elem.Jid(), What: what.Value, Data: "queued"}
					switch source {
					case "command":
						elem.SetValue(want.Data)
					case "event":
						rq.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.Input, Data: want.Data}
						// The callback waits while the processing loop becomes idle.
						synctest.Wait()
						release <- struct{}{}
					case "dirty":
						rq.Dirty(elem)
						rq.Jaws.distributeDirt()
					case "reload":
						want = wire.WsMsg{What: what.Reload}
						rq.Reload()
					case "maintenance":
						// Model queued output without a pending notification so the
						// normal maintenance tick must supply the fallback wake-up.
						rq.muQueue.Lock()
						rq.wsQueue = append(rq.wsQueue, want)
						rq.muQueue.Unlock()
						synctest.Wait()
						select {
						case msg := <-rq.OutCh:
							t.Fatalf("output before maintenance: %v", msg)
						default:
						}
						time.Sleep(time.Second)
					}
					synctest.Wait()
					select {
					case got := <-rq.OutCh:
						if got != want {
							t.Fatalf("output = %v, want %v", got, want)
						}
					default:
						t.Fatal("queued work did not wake the idle processing loop")
					}
				}
			})
		})
	}
}
