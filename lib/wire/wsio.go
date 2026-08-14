package wire

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// writeBatchLimit is the maximum number of bytes WriteLoop coalesces into a
// single outbound WebSocket text message before flushing.
const writeBatchLimit = 32 * 1024

// ReadLoop reads WebSocket text messages and sends each valid protocol record
// on incomingMsgCh.
//
// Records are LF-terminated and delivered in order. A text message may contain
// multiple records; malformed records are skipped independently.
//
// A WebSocket read that remains pending for idleInterval triggers a ping bounded
// by pingTimeout. A successful pong starts another idle interval for the pending
// read. Time spent parsing or delivering an already-read message is not idle
// time. Data processed while a ping is pending supersedes a failed ping.
// idleInterval and pingTimeout must be greater than zero; non-positive values
// are invalid.
//
// Closes incomingMsgCh on exit.
//
// Canceling ctx or closing doneCh interrupts reads and pings in progress and is
// not reported through ccf.
//
// ccf may be nil, in which case errors are not reported and only the loop exits.
func ReadLoop(ctx context.Context, ccf context.CancelCauseFunc, doneCh <-chan struct{}, incomingMsgCh chan<- WsMsg, idleInterval, pingTimeout time.Duration, ws *websocket.Conn) {
	ctx, cancel := contextWithDone(ctx, doneCh)
	readResultCh := make(chan wsReadResult)
	pingResultCh := make(chan error, 1)
	var workers sync.WaitGroup
	// coder/websocket requires a Reader to run concurrently with Ping. Keeping
	// socket reads in one worker lets local delivery pause the idle timer while a
	// pending read still handles control frames. The unbuffered channel bounds
	// read-ahead during delivery to one complete WebSocket message.
	workers.Go(func() { readWebSocket(ctx, readResultCh, ws) })

	idleTimer := time.NewTimer(idleInterval)
	idleTimerCh := idleTimer.C
	armIdleTimer := func() {
		idleTimer.Reset(idleInterval)
		idleTimerCh = idleTimer.C
	}
	var activityDuringPing bool

	defer func() {
		cancel()
		workers.Wait()
		idleTimer.Stop()
		close(incomingMsgCh)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case result := <-readResultCh:
			// A nil timer channel denotes the one in-flight ping while this select is
			// active. Keep it nil so parsing and delivery do not count as read-idle;
			// Reset afterward discards any expiry that occurs meanwhile.
			activityDuringPing = idleTimerCh == nil
			idleTimerCh = nil
			if result.err != nil {
				reportError(ctx, doneCh, ccf, result.err)
				return
			}
			if result.typ == websocket.MessageText {
				for record := range bytes.Lines(result.txt) {
					if msg, parsed := Parse(record); parsed {
						select {
						case <-ctx.Done():
							return
						case incomingMsgCh <- msg:
						}
					}
				}
			}
			if !activityDuringPing {
				armIdleTimer()
			}
		case <-idleTimerCh:
			idleTimerCh = nil
			// Ping waits for its pong, so it must not delay data that arrives while
			// the probe is in flight.
			workers.Go(func() {
				pingctx, pingcancel := context.WithTimeout(ctx, pingTimeout)
				err := ws.Ping(pingctx)
				pingcancel()
				pingResultCh <- err
			})
		case err := <-pingResultCh:
			if err != nil && !activityDuringPing {
				reportError(ctx, doneCh, ccf, err)
				return
			}
			activityDuringPing = false
			armIdleTimer()
		}
	}
}

type wsReadResult struct {
	typ websocket.MessageType
	txt []byte
	err error
}

func readWebSocket(ctx context.Context, resultCh chan<- wsReadResult, ws *websocket.Conn) {
	for {
		typ, txt, err := ws.Read(ctx)
		select {
		case <-ctx.Done():
			return
		case resultCh <- wsReadResult{typ: typ, txt: txt, err: err}:
		}
		if err != nil {
			return
		}
	}
}

// WriteLoop formats messages read from outboundMsgCh and writes them to the
// WebSocket.
//
// Consecutive queued records may be coalesced into one text message.
//
// Each write is bounded by writeTimeout, which must be positive.
//
// Closes the WebSocket on exit.
//
// Canceling ctx or closing doneCh interrupts writes in progress and is not
// reported through ccf.
//
// ccf may be nil, in which case errors are not reported and only the loop exits.
func WriteLoop(ctx context.Context, ccf context.CancelCauseFunc, doneCh <-chan struct{}, outboundMsgCh <-chan WsMsg, writeTimeout time.Duration, ws *websocket.Conn) {
	defer func() { _ = ws.Close(websocket.StatusNormalClosure, "") }()
	ctx, cancel := contextWithDone(ctx, doneCh)
	defer cancel()
	var err error
	for err == nil {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-outboundMsgCh:
			if !ok {
				return
			}
			writectx, writecancel := context.WithTimeout(ctx, writeTimeout)
			var wc io.WriteCloser
			if wc, err = ws.Writer(writectx, websocket.MessageText); err == nil {
				err = writeData(wc, msg, outboundMsgCh)
			}
			writecancel()
		}
	}
	reportError(ctx, doneCh, ccf, err)
}

func contextWithDone(ctx context.Context, doneCh <-chan struct{}) (ioctx context.Context, cancel context.CancelFunc) {
	ioctx, cancel = context.WithCancel(ctx)
	go func() {
		select {
		case <-doneCh:
			cancel()
		case <-ioctx.Done():
		}
	}()
	return
}

func reportError(ctx context.Context, doneCh <-chan struct{}, ccf context.CancelCauseFunc, err error) {
	if ccf != nil {
		// Check doneCh directly because contextWithDone propagates it asynchronously.
		select {
		case <-ctx.Done():
		case <-doneCh:
		default:
			ccf(err)
		}
	}
}

func writeData(wc io.WriteCloser, firstMsg WsMsg, outboundMsgCh <-chan WsMsg) (err error) {
	b := firstMsg.Append(nil)
	// accumulate data to send as long as more messages are available until it
	// exceeds writeBatchLimit
batchloop:
	for len(b) < writeBatchLimit {
		select {
		case msg, ok := <-outboundMsgCh:
			if !ok {
				break batchloop
			}
			b = msg.Append(b)
		default:
			break batchloop
		}
	}
	_, err = wc.Write(b)
	err = errors.Join(err, wc.Close())
	return
}
