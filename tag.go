package jaws

import (
	"fmt"
	"html/template"
	"reflect"
)

type Tag struct{ Value interface{} }

func TagString(tag interface{}) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		} else if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}

func TagExpand(rq *Request, tag interface{}, result []interface{}) []interface{} {
	if len(result) > 1000 {
		panic("jaws: too many tags")
	}
	switch data := tag.(type) {
	case string:
	case template.HTML:
	case int:
	case int8:
	case int16:
	case int32:
	case int64:
	case uint:
	case uint8:
	case uint16:
	case uint32:
	case uint64:
	case float32:
	case float64:
	case bool:
	case error:
	case []string:
	case []template.HTML:

	case nil:
		return result
	case atomicGetter:
		return append(result, data.v)
	case []Tag:
		for _, v := range data {
			result = append(result, v)
		}
		return result
	case []interface{}:
		for _, v := range data {
			result = TagExpand(rq, v, result)
		}
		return result
	case TagGetter:
		return TagExpand(rq, data.JawsGetTag(rq), result)
	default:
		return append(result, data)
	}
	panic("jaws: not allowed as a tag: " + TagString(tag))
}
