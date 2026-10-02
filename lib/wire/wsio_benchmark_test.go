package wire

import (
	"context"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/what"
)

// BenchmarkWriteLoop measures one buffered record per WebSocket message.
func BenchmarkWriteLoop(b *testing.B) {
	client, server := pipe(b)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()
	doneCh := make(chan struct{})
	outCh := make(chan WsMsg)
	loopDone := make(chan struct{})
	go func() {
		WriteLoop(ctx, nil, doneCh, outCh, time.Hour, server)
		close(loopDone)
	}()

	msg := WsMsg{What: what.Alert, Data: "hello"}
	b.ReportAllocs()
	for b.Loop() {
		outCh <- msg
		if _, _, err := client.Read(ctx); err != nil {
			b.Fatal(err)
		}
	}
	close(doneCh)
	_ = client.CloseNow()
	<-loopDone
}
