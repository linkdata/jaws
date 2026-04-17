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

type visitFrame struct {
	key  visitKey
	node any
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

func appendResult(result *[]any, tag any) error {
	*result = append(*result, tag)
	if len(*result) > 100 {
		return ErrTooManyTags
	}
	return nil
}

func appendActiveCycle(result *[]any, active []visitFrame, start int) error {
	for _, frame := range active[start:] {
		if _, ok := frame.node.(TagGetter); ok {
			if err := NewErrNotUsableAsTag(frame.node); err != nil {
				return err
			}
			if err := appendResult(result, frame.node); err != nil {
				return err
			}
		}
	}
	return nil
}

func withActive(node any, active []visitFrame, activeIndex map[visitKey]int, onCycle func([]visitFrame, int) error, fn func([]visitFrame) error) error {
	key, ok := makeVisitKey(node)
	if !ok {
		return fn(active)
	}
	if idx, ok := activeIndex[key]; ok {
		return onCycle(active, idx)
	}
	activeIndex[key] = len(active)
	active = append(active, visitFrame{key: key, node: node})
	defer func() {
		delete(activeIndex, key)
	}()
	return fn(active)
}

func expand_ai(depth int, ctx Context, tag any, result *[]any, active []visitFrame, activeIndex map[visitKey]int) error {
	if depth > 10 || len(*result) > 100 {
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
			if err := appendResult(result, v); err != nil {
				return err
			}
		}
		return nil
	case TagGetter:
		newTag := data.JawsGetTag(ctx)
		if same, err := sameTagGetterTag(data, newTag); err != nil {
			return err
		} else if same {
			return appendResult(result, data)
		}
		return withActive(data, active, activeIndex, func(active []visitFrame, start int) error {
			return appendActiveCycle(result, active, start)
		}, func(active []visitFrame) error {
			return expand_ai(depth+1, ctx, newTag, result, active, activeIndex)
		})
	case []any:
		return withActive(data, active, activeIndex, func(active []visitFrame, start int) error {
			return appendActiveCycle(result, active, start)
		}, func(active []visitFrame) error {
			for _, v := range data {
				if err := expand_ai(depth+1, ctx, v, result, active, activeIndex); err != nil {
					return err
				}
			}
			return nil
		})
	default:
		if err := NewErrNotUsableAsTag(data); err != nil {
			return err
		}
		return appendResult(result, data)
	}
	return errIllegalTagType{tag: tag}
}

func expand(depth int, ctx Context, tag any, m map[any]struct{}, x map[TagGetter]any) (err error) {
	if depth > 10 || len(m) >= 100 {
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
			if err = expand(depth+1, ctx, v, m, x); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for _, v := range data {
			if err = expand(depth+1, ctx, v, m, x); err != nil {
				return err
			}
		}
		return nil
	case TagGetter:
		newTag := data.JawsGetTag(ctx)
		if NewErrNotComparable(data) == nil {
			if _, ok := x[data]; ok {
				return
			}
			m[data] = struct{}{}
			x[data] = struct{}{}
		}
		if NewErrNotComparable(newTag) == nil {
			if _, ok := m[newTag]; ok {
				return
			}
			m[newTag] = struct{}{}
			x[data] = newTag
		}
		if reflect.TypeOf(data) == reflect.TypeOf(newTag) {
			if err = NewErrNotUsableAsTag(newTag); err != nil {
				return
			}
			if data == newTag {
				m[newTag] = struct{}{}
				return nil
			}
		}
		return expand(depth+1, ctx, newTag, m, x)
	default:
		if err = NewErrNotUsableAsTag(data); err == nil {
			m[tag] = struct{}{}
		}
		return
	}
	return errIllegalTagType{tag: tag}
}

func TagExpand(ctx Context, tag any) (result []any, err error) {
	// err := expand_ai(0, ctx, tag, &result, nil, map[visitKey]int{})
	m := make(map[any]struct{})
	x := make(map[TagGetter]any)
	if err = expand(0, ctx, tag, m, x); err == nil {
		for k := range m {
			if tg, ok := k.(TagGetter); ok {
				if _, ok := m[x[tg]]; ok {
					continue
				}
			}
			result = append(result, k)
		}
		// result = slices.Collect(maps.Keys(m))
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
