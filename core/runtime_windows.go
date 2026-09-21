//go:build windows

package core

import "errors"

func newWindow(opts Options) (Window, error) {
	return nil, errors.New("nib: windows (WebView2) support not implemented yet")
}
