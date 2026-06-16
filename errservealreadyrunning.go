package jaws

import "errors"

// ErrServeAlreadyRunning indicates the JaWS processing loop is already running.
var ErrServeAlreadyRunning = errors.New("serve loop already running")
