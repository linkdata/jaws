package tag

// ErrTooManyTags is returned when tag expansion exceeds the recursion depth
// ([maxTagDepth]) or result count ([maxTagCount]) limits.
var ErrTooManyTags errTooManyTags

// errTooManyTags is intentionally fieldless: every instance equals the
// [ErrTooManyTags] sentinel, so errors.Is matches via the == comparison and no
// Is method is needed (unlike the field-carrying typed errors in this package).
// If a field is ever added, add an Is method matching against ErrTooManyTags.
type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}
