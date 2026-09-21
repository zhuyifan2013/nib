//go:build linux

package core

import "errors"

func newWindow(opts Options) (Window, error) {
	return nil, errors.New("nib: linux (WebKitGTK) support not implemented yet")
}
