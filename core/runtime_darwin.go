//go:build darwin

package core

import "nib.dev/nib/webview"

func newWindow(opts Options) (Window, error) {
	return webview.New(webview.Options{
		Title:    opts.Title,
		Width:    opts.Width,
		Height:   opts.Height,
		DevTools: opts.DevTools,
	}), nil
}
