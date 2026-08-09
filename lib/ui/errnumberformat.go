package ui

import (
	"errors"
	"fmt"
)

// ErrNumberFormat indicates that a Number codec violated its formatting contract.
var ErrNumberFormat = errors.New("invalid number format")

func newErrNumberFormat(detail string) error {
	return fmt.Errorf("%w: %s", ErrNumberFormat, detail)
}
