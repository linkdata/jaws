package tag

import (
	"fmt"
	"html/template"
	"reflect"
)

type Tag string

func TagString(tag any) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		} else if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}

type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}

var ErrTooManyTags = errTooManyTags{}

func sameTagGetterTag(owner TagGetter, tag any) (same bool, err error) {
	if reflect.TypeOf(owner) != reflect.TypeOf(tag) {
		return false, nil
	}
	if err = NewErrNotUsableAsTag(tag); err != nil {
		return false, err
	}
	return any(owner) == tag, nil
}

type visitKey struct {
	value any
	t     reflect.Type
	ptr   uintptr
}

func makeVisitKey(node any) (key visitKey, ok bool) {
	if node == nil {
		return key, false
	}
	t := reflect.TypeOf(node)
	if t.Comparable() {
		return visitKey{value: node}, true
	}
	v := reflect.ValueOf(node)
	switch v.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return visitKey{t: t, ptr: v.Pointer()}, true
	default:
		return key, false
	}
}

func addResult(result map[any]struct{}, tag any) error {
	result[tag] = struct{}{}
	if len(result) > 100 {
		return ErrTooManyTags
	}
	return nil
}

func addActiveCycle(result map[any]struct{}, active []any, start int) error {
	for _, node := range active[start:] {
		if _, ok := node.(TagGetter); ok {
			if err := NewErrNotUsableAsTag(node); err != nil {
				return err
			}
			if err := addResult(result, node); err != nil {
				return err
			}
		}
	}
	return nil
}

func withActive(node any, active []any, activeIndex map[visitKey]int, onCycle func([]any, int) error, fn func([]any) error) error {
	key, ok := makeVisitKey(node)
	if !ok {
		return fn(active)
	}
	if idx, ok := activeIndex[key]; ok {
		return onCycle(active, idx)
	}
	activeIndex[key] = len(active)
	active = append(active, node)
	defer func() {
		delete(activeIndex, key)
	}()
	return fn(active)
}

func expand(depth int, ctx Context, tag any, result map[any]struct{}, active []any, activeIndex map[visitKey]int) error {
	if depth > 10 || len(result) > 100 {
		return ErrTooManyTags
	}
	switch data := tag.(type) {
	case string:
	case template.HTML:
	case template.HTMLAttr:
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
	case nil:
		return nil
	case []Tag:
		for _, v := range data {
			if err := addResult(result, v); err != nil {
				return err
			}
		}
		return nil
	case TagGetter:
		newTag := data.JawsGetTag(ctx)
		if same, err := sameTagGetterTag(data, newTag); err != nil {
			return err
		} else if same {
			return addResult(result, data)
		}
		return withActive(data, active, activeIndex, func(active []any, start int) error {
			return addActiveCycle(result, active, start)
		}, func(active []any) error {
			return expand(depth+1, ctx, newTag, result, active, activeIndex)
		})
	case []any:
		return withActive(data, active, activeIndex, func(active []any, start int) error {
			return addActiveCycle(result, active, start)
		}, func(active []any) error {
			for _, v := range data {
				if err := expand(depth+1, ctx, v, result, active, activeIndex); err != nil {
					return err
				}
			}
			return nil
		})
	default:
		if err := NewErrNotUsableAsTag(data); err != nil {
			return err
		}
		return addResult(result, data)
	}
	return errIllegalTagType{tag: tag}
}

func TagExpand(ctx Context, tag any) (result []any, err error) {
	seen := make(map[any]struct{})
	if err = expand(0, ctx, tag, seen, nil, map[visitKey]int{}); err == nil {
		for tagValue := range seen {
			result = append(result, tagValue)
		}
	}
	return result, err
}

func MustTagExpand(ctx Context, tag any) []any {
	result, err := TagExpand(ctx, tag)
	if err != nil {
		if ctx != nil {
			ctx.MustLog(err)
		} else {
			panic(err)
		}
	}
	return result
}
