//go:build !darwin && !windows && !linux

package core

import "errors"

func newWindow(opts Options) (Window, error) {
	return nil, errors.New("nib: unsupported platform")
}
