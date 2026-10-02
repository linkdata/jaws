package tag

// ErrTooManyTags reports that tag expansion exceeded a depth or count limit.
//
// Expansion allows at most 10 nested levels and 100 unique tags. A count
// failure can return 101 partial entries alongside the error.
var ErrTooManyTags errTooManyTags

// errTooManyTags is intentionally fieldless: every instance equals the
// [ErrTooManyTags] sentinel, so errors.Is matches via the == comparison and no
// Is method is needed (unlike the field-carrying typed errors in this package).
// If a field is ever added, add an Is method matching against ErrTooManyTags.
type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}
