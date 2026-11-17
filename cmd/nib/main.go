// nib is the Nib development and diagnostic CLI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"nib.dev/nib/cli"
	"nib.dev/nib/driver"
	"nib.dev/nib/harness"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, errOut io.Writer) int {
	globalJSON := false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "--json=true" || args[0] == "--json=false") {
		globalJSON = args[0] != "--json=false"
		args = args[1:]
	}
	command := "nib"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonMode := fs.Bool("json", globalJSON, "emit nib.cli/v1 JSON/NDJSON")
	help := fs.Bool("help", false, "show help")
	fs.BoolVar(help, "h", false, "show help")
	var noWatch, headless *bool
	var interval, driverTimeout *time.Duration
	var eventPath, sock, nav, evalAfter *string
	var port *int
	switch command {
	case "dev":
		noWatch = fs.Bool("nowatch", false, "build once without watching")
		interval = fs.Duration("interval", 250*time.Millisecond, "file polling interval")
		fallthrough
	case "shell":
		headless = fs.Bool("headless", false, "hide window (GUI session still required)")
		port = fs.Int("driver-port", 0, "authenticated local driver port")
		driverTimeout = fs.Duration("driver-timeout", 0, "driver timeout, 0=5s default, maximum30s")
		eventPath = fs.String("events", "", "NDJSON events path (- means stdout in human mode)")
		if command == "shell" {
			sock = fs.String("sock", "", "unix socket path")
		}
	case "probe":
		sock = fs.String("sock", "", "shell unix socket path")
		nav = fs.String("navigate", "<html><body><h1>nib probe</h1></body></html>", "starting page")
		evalAfter = fs.String("evalafter", "", "enable delayed bridge inspection")
	case "doctor", "nib":
	default:
		r := &cli.Reporter{JSON: cli.WantsJSON(fs, args), Command: command, Out: out, Err: errOut}
		return r.Fail("cli/unknown_command", "unknown command: "+command, "Use nib dev, shell, probe, or doctor.", cli.ExitUsage)
	}
	r := &cli.Reporter{JSON: cli.WantsJSON(fs, args), Command: command, Out: out, Err: errOut}
	if err := cli.Parse(fs, args); err != nil {
		return r.Fail("cli/invalid_flag", err.Error(), "Use --help to inspect supported flags.", cli.ExitUsage)
	}
	r.JSON = *jsonMode
	if *help {
		var b stringsBuilder
		fs.SetOutput(&b)
		fs.PrintDefaults()
		if command == "nib" {
			b.b = append([]byte("Commands: dev, shell, probe, doctor\nUse nib <command> --help for command options.\n"), b.b...)
		}
		if !r.JSON {
			fmt.Fprintln(out, "nib "+command+" options:\n"+b.String())
		}
		return r.Finish(map[string]any{"usage": command, "flags": b.String()})
	}
	failArgs := func(message string) int {
		return r.Fail("cli/invalid_args", message, "Use --help; dev requires one package, shell/probe require --sock.", cli.ExitUsage)
	}
	if command == "nib" {
		return failArgs("a command is required")
	}
	expected := 0
	if command == "dev" {
		expected = 1
	}
	if fs.NArg() != expected {
		return failArgs(fmt.Sprintf("%s requires %d positional arguments", command, expected))
	}
	if sock != nil && *sock == "" {
		return failArgs("--sock is required")
	}
	if interval != nil && *interval <= 0 {
		return failArgs("--interval must be positive")
	}
	if driverTimeout != nil {
		if err := driver.ValidateTimeout(*driverTimeout); err != nil {
			return failArgs(err.Error())
		}
		if *port < 0 || *port > 65535 {
			return failArgs("--driver-port must be 0..65535")
		}
		if *port != 0 && len(os.Getenv("NIB_DRIVER_TOKEN")) < 16 {
			return failArgs("NIB_DRIVER_TOKEN must contain at least 16 bytes")
		}
	}
	if eventPath != nil && r.JSON && *eventPath == "-" {
		return r.Fail("cli/output_conflict", "--json cannot be combined with --events -", "Write events to a file using --events <path>.", cli.ExitUsage)
	}
	logs := cli.NewLogWriter(errOut)
	defer logs.Flush()
	previous := log.Writer()
	log.SetOutput(logs)
	defer log.SetOutput(previous)
	if command == "doctor" {
		report, exit := cli.Doctor()
		if !r.JSON {
			for _, c := range report.Checks {
				fmt.Fprintf(out, "%-16s %-10s %s\n", c.ID, c.Status, c.Evidence)
			}
		}
		if exit == cli.ExitOK {
			return r.Finish(report)
		}
		code := "doctor/dependencies"
		hint := "Install missing dependencies reported by checks."
		if exit == cli.ExitUnsupported {
			code = "doctor/not_implemented"
			hint = "Use macOS; Windows/Linux runtime implementation is pending."
		}
		r.Emit("error", "failed", report, &cli.Problem{Code: code, Message: report.Overall, Hint: hint}, &exit)
		return exit
	}
	r.Lifecycle("starting", nil)
	var err error
	switch command {
	case "dev":
		childOut := io.Writer(cli.NewLogWriter(out))
		if lw, ok := childOut.(*cli.LogWriter); ok {
			defer lw.Flush()
		}
		if r.JSON {
			childOut = logs
		}
		err = harness.Dev(harness.DevOptions{Pkg: fs.Arg(0), NoWatch: *noWatch, Interval: *interval, Events: *eventPath, Headless: *headless, Driver: driver.Options{Port: *port, Timeout: *driverTimeout, Token: os.Getenv("NIB_DRIVER_TOKEN")}, ChildOutput: childOut, ChildError: logs, Notify: r.Lifecycle})
	case "shell":
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		err = harness.RunShellWithOptions(*sock, *eventPath, harness.ShellOptions{Headless: *headless, Driver: driver.Options{Port: *port, Timeout: *driverTimeout, Token: os.Getenv("NIB_DRIVER_TOKEN")}, Context: ctx, Notify: r.Lifecycle})
	case "probe":
		err = runProbe(*sock, *nav, *evalAfter, out, logs, r)
	}
	if err != nil {
		var failure *cli.Failure
		if errors.As(err, &failure) {
			return r.Fail(failure.Code, failure.Error(), failure.Hint, failure.Exit)
		}
		return r.Fail(command+"/runtime", err.Error(), "Inspect stderr diagnostics and run nib doctor.", cli.ExitFailure)
	}
	r.Lifecycle("stopped", nil)
	return r.Finish(map[string]any{"stopped": true})
}

// Keeps help text local without printing directly to machine stdout.
type stringsBuilder struct{ b []byte }

func (b *stringsBuilder) Write(p []byte) (int, error) { b.b = append(b.b, p...); return len(p), nil }
func (b *stringsBuilder) String() string              { return string(b.b) }
