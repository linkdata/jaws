package core

import (
	"bytes"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestJaws_GenerateHeadHTMLConcurrentWithHeadHTML(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if err := jw.GenerateHeadHTML("/a.js", "/b.css"); err != nil {
					t.Error(err)
					return
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
				var buf bytes.Buffer
				if err := rq.HeadHTML(&buf); err != nil {
					t.Error(err)
				}
				jw.recycle(rq)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
