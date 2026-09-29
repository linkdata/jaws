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
	<-tr.ReadyCh

	tr.Close()
	for range tr.OutCh {
	}
	<-tr.DoneCh

	fmt.Println("stopped")
	// Output: stopped
}
