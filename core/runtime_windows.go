//go:build windows

package core

import (
	"errors"

	"nib.dev/nib/events"
)

func newWindow(opts Options, bus *events.Bus) (Window, error) {
	return nil, errors.New("nib: windows (WebView2) support not implemented yet")
}
