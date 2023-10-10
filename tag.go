package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
)

type Tag string

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

var ErrTooManyTags = errors.New("too many tags")

func tagExpand(l int, rq *Request, tag interface{}, result []interface{}) []interface{} {
	if l > 10 || len(result) > 100 {
		panic(ErrTooManyTags)
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
	case []Tag:
		for _, v := range data {
			result = append(result, v)
		}
		return result
	case TagGetter:
		return tagExpand(l+1, rq, data.JawsGetTag(rq), result)
	case []interface{}:
		for _, v := range data {
			result = tagExpand(l+1, rq, v, result)
		}
		return result
	default:
		return append(result, data)
	}
	panic("jaws: not allowed as a tag: " + TagString(tag))
}

func TagExpand(rq *Request, tag interface{}, result []interface{}) []interface{} {
	return tagExpand(0, rq, tag, result)
}
