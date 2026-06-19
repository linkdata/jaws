// Package bind adapts Go values to JaWS getter, setter, HTML and tag
// interfaces.
//
// [New] is the centerpiece: it binds a pointer guarded by a [sync.Locker] to
// the JaWS getter, setter and event interfaces and returns a [Binder] whose
// behavior can be extended with chained hooks ([Binder.SetLocked],
// [Binder.GetLocked], [Binder.GetHTML], [Binder.Success], [Binder.Clicked] and
// the others), each returning a new, concurrency-safe [Binder] that wraps the
// previous one.
//
// The remaining constructors accept either existing bind interfaces or
// static values and return small adapters that can be used by package ui
// widgets. They panic when called with a value whose dynamic type does not match
// the requested adapter type, so use them at trusted construction points rather
// than on unvalidated external input.
package bind
