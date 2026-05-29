package tag

// ErrTooManyTags is returned when tag expansion exceeds the recursion depth
// ([maxTagDepth]) or result count ([maxTagCount]) limits.
var ErrTooManyTags errTooManyTags

type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}
