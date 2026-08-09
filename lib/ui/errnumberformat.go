package ui

import (
	"errors"
	"fmt"
)

// ErrNumberFormat indicates that a Number binding violated its codec or value contract.
var ErrNumberFormat = errors.New("invalid number format")

func newErrNumberFormat(detail string) error {
	return fmt.Errorf("%w: %s", ErrNumberFormat, detail)
}
