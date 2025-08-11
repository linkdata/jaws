package jaws

// DeepEqualler can be used by JawsSet to check for value changes.
type DeepEqualler[T comparable] interface {
	// DeepEqual should return true if the values are logically the same.
	DeepEqual(other T) (yes bool)
}
