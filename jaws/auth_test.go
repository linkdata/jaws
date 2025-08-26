package jaws

import (
	"testing"
)

func Test_defaultAuth(t *testing.T) {
	a := defaultAuth{}
	if a.Data() != nil {
		t.Fatal()
	}
	if a.Email() != "" {
		t.Fatal()
	}
	if a.IsAdmin() != true {
		t.Fatal()
	}
}
