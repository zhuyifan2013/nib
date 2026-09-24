//go:build !darwin && !windows && !linux

package core

import (
	"errors"

	"nib.dev/nib/events"
)

func newWindow(opts Options, bus *events.Bus) (Window, error) {
	return nil, errors.New("nib: unsupported platform")
}
