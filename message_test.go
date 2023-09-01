package jaws

import (
	"fmt"
	"testing"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

func Test_Message_String(t *testing.T) {
	is := is.New(t)
	msg := Message{
		Tags: []interface{}{"Elem"},
		What: what.None,
		Data: "Data\nText",
	}
	is.Equal(msg.String(), "{[\"Elem\"], None, \"Data\\nText\", <nil>}")
	const jawsKey = uint64(0xcafebabe)
	msg.from = &Request{JawsKey: jawsKey}
	keyStr := JawsKeyString(jawsKey)
	keyVal := JawsKeyValue(keyStr)
	is.Equal(keyVal, jawsKey)
	is.Equal(uint64(0), JawsKeyValue(""))
	is.Equal(msg.String(), fmt.Sprintf("{[\"Elem\"], None, \"Data\\nText\", Request<%s>}", keyStr))
	msg.from = &Request{JawsKey: 0}
	is.Equal(msg.String(), "{[\"Elem\"], None, \"Data\\nText\", Request<>}")
}
