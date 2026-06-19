package jawstest_test

import (
	"fmt"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
)

func ExampleNewTestRequest() {
	jw, err := jaws.New()
	if err != nil {
		panic(err)
	}
	defer jw.Close()
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		panic("request was not created")
	}
	<-tr.ReadyCh

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			select {
			case <-tr.OutCh:
			case <-tr.DoneCh:
				return
			}
		}
	}()

	tr.Close()
	<-tr.DoneCh
	<-drained

	fmt.Println("stopped")
	// Output: stopped
}
