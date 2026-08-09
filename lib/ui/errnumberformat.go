package ui

import (
	"errors"
	"fmt"
)

// ErrNumberFormat identifies a violation of a [NumberCodec] or bound-value requirement.
var ErrNumberFormat = errors.New("invalid number format")

func newErrNumberFormat(detail string) error {
	return fmt.Errorf("%w: %s", ErrNumberFormat, detail)
}
