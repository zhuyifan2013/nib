package cli

import (
	"context"
	"encoding/json"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Check struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
	Hint     string `json:"hint,omitempty"`
}
type DoctorReport struct {
	OS      string  `json:"os"`
	Arch    string  `json:"arch"`
	Runtime string  `json:"runtime"`
	Overall string  `json:"overall"`
	Checks  []Check `json:"checks"`
}

func Doctor() (DoctorReport, int) {
	report := DoctorReport{OS: runtime.GOOS, Arch: runtime.GOARCH, Runtime: "not_implemented", Overall: "checks_passed"}
	add := func(id, status, evidence, hint string) {
		report.Checks = append(report.Checks, Check{id, status, evidence, hint})
		if status == "missing" {
			report.Overall = "missing_dependencies"
		}
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		add("go", "missing", "go executable not found", "Install Go 1.23 or later and add it to PATH.")
	} else {
		output, err := doctorCommand(goPath, "env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "CC")
		var env map[string]string
		if err != nil || json.Unmarshal([]byte(output), &env) != nil {
			add("go", "missing", "go env failed", "Check your Go installation and environment.")
		} else {
			goVersion := env["GOVERSION"]
			if !version.IsValid(goVersion) {
				add("go", "unknown", goVersion, "Check that the Go toolchain meets the Go 1.23 minimum.")
			} else if version.Compare(goVersion, "go1.23") < 0 {
				add("go", "missing", goVersion, "Install Go 1.23 or later.")
			} else {
				add("go", "available", goVersion, "")
			}
			if env["GOOS"] != runtime.GOOS || env["GOARCH"] != runtime.GOARCH {
				add("target", "missing", env["GOOS"]+"/"+env["GOARCH"], "Use native GOOS/GOARCH for local CGO desktop builds.")
			} else {
				add("target", "available", env["GOOS"]+"/"+env["GOARCH"], "")
			}
			if env["CGO_ENABLED"] != "1" {
				add("cgo", "missing", env["CGO_ENABLED"], "Set CGO_ENABLED=1 and install a native C compiler.")
			} else {
				add("cgo", "available", "CGO_ENABLED=1", "")
			}
			cc := strings.Fields(env["CC"])
			if len(cc) == 0 {
				add("compiler", "missing", "CC is empty", "Install a native C compiler.")
			} else if out, err := doctorCommand(cc[0], append(cc[1:], "--version")...); err != nil {
				add("compiler", "missing", "configured compiler cannot execute", "Install the compiler configured by go env CC.")
			} else {
				add("compiler", "available", strings.Split(out, "\n")[0], "")
			}
		}
	}
	switch runtime.GOOS {
	case "darwin":
		report.Runtime = "implemented"
		sdk, err := doctorCommand("xcrun", "--show-sdk-path")
		if err != nil {
			add("macos_sdk", "missing", "xcrun could not locate a macOS SDK", "Install Xcode Command Line Tools with xcode-select --install.")
		} else {
			add("macos_sdk", "available", sdk, "")
			for _, framework := range []string{"Cocoa", "WebKit"} {
				path := filepath.Join(sdk, "System/Library/Frameworks", framework+".framework")
				if _, err := os.Stat(path); err != nil {
					add(strings.ToLower(framework), "missing", framework+" framework missing from selected SDK", "Select a complete macOS SDK with xcode-select.")
				} else {
					add(strings.ToLower(framework), "available", path, "")
				}
			}
		}
		add("gui_session", "unknown", "GUI/WindowServer and WebView launch were not exercised", "Run scripts/driver_e2e.py in a logged-in macOS GUI session.")
	case "linux":
		out, err := doctorCommand("pkg-config", "--modversion", "webkit2gtk-4.1")
		if err != nil {
			out, err = doctorCommand("pkg-config", "--modversion", "webkit2gtk-4.0")
		}
		if err != nil {
			add("webkitgtk", "unknown", "pkg-config did not find WebKitGTK", "Linux runtime is not implemented; do not infer support from dependency presence.")
		} else {
			add("webkitgtk", "available", out, "")
		}
	case "windows":
		add("webview2", "unknown", "WebView2 runtime was not probed", "Windows runtime is not implemented.")
	}
	if report.Runtime == "not_implemented" {
		report.Overall = "not_implemented"
		return report, ExitUnsupported
	}
	if report.Overall == "missing_dependencies" {
		return report, ExitFailure
	}
	return report, ExitOK
}
func doctorCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}
