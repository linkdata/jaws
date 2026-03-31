package jaws

import (
	"io"
	"testing"
	"time"

	"github.com/linkdata/jaws/core/wire"
)

func nextBroadcast(t *testing.T, jw *Jaws) wire.Message {
	t.Helper()
	select {
	case msg := <-jw.bcastCh:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
		return wire.Message{}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.EOF
}
