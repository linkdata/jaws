//go:build go1.18
// +build go1.18

package jaws

func ListMove[T comparable](l []T, elem T, steps int) (changed bool) {
	for i := len(l) - 1; steps < 0 && i > 0; i-- {
		if l[i] == elem {
			l[i-1], l[i] = l[i], l[i-1]
			steps++
			changed = true
		}
	}
	for i := 0; steps > 0 && i < len(l)-1; i++ {
		if l[i] == elem {
			l[i+1], l[i] = l[i], l[i+1]
			steps--
			changed = true
		}
	}
	return
}

func ListRemove[T comparable](l []T, e T) []T {
	for i, v := range l {
		if v == e {
			j := i
			for i++; i < len(l); i++ {
				v = l[i]
				if v != e {
					l[j] = v
					j++
				}
			}
			return l[:j]
		}
	}
	return l
}

func ListOrder[T comparable](l []T, jw *Jaws) {
	tags := make([]interface{}, len(l))
	for i := range l {
		tags[i] = l[i]
	}
	jw.Order(tags)
}
