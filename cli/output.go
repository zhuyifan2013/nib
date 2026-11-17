// Package cli defines Nib's versioned machine output and shared argument handling.
package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

const Schema = "nib.cli/v1"
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitUnsupported = 3
)

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}
type Record struct {
	Schema   string   `json:"schema"`
	Command  string   `json:"command"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Data     any      `json:"data,omitempty"`
	Error    *Problem `json:"error,omitempty"`
	ExitCode *int     `json:"exit_code,omitempty"`
}
type Reporter struct {
	JSON     bool
	Command  string
	Out, Err io.Writer
	mu       sync.Mutex
	writeErr error
}

func (r *Reporter) Emit(kind, status string, data any, p *Problem, exit *int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.JSON {
		var sanitizedData any
		if data != nil {
			encoded, err := json.Marshal(data)
			if err != nil {
				r.writeErr = err
				return
			}
			sanitizedData = json.RawMessage(redactJSON(encoded))
		}
		var sanitizedProblem *Problem
		if p != nil {
			sanitizedProblem = &Problem{Code: p.Code, Message: Redact(p.Message), Hint: Redact(p.Hint)}
		}
		command := r.Command
		switch command {
		case "nib", "nibgen", "dev", "shell", "probe", "doctor":
		default:
			command = Redact(command)
		}
		raw, err := json.Marshal(Record{Schema: Schema, Command: command, Kind: kind, Status: status, Data: sanitizedData, Error: sanitizedProblem, ExitCode: exit})
		if err != nil {
			r.writeErr = err
			return
		}
		_, r.writeErr = fmt.Fprintln(r.Out, string(raw))
		return
	}
	if p != nil {
		fmt.Fprintf(r.Err, "%s: %s\n%s\n", Redact(r.Command), Redact(p.Message), Redact(p.Hint))
	}
}

// redactJSON replaces secrets within individual JSON strings, preserving the
// document structure even when a secret contains quotes or spans fields.
func redactJSON(raw []byte) string {
	if os.Getenv("NIB_DRIVER_TOKEN") == "" {
		return string(raw)
	}
	var result bytes.Buffer
	for i := 0; i < len(raw); {
		if raw[i] != '"' {
			result.WriteByte(raw[i])
			i++
			continue
		}
		start := i
		i++
		for i < len(raw) {
			if raw[i] == '\\' {
				i += 2
				continue
			}
			if raw[i] == '"' {
				i++
				break
			}
			i++
		}
		var value string
		if err := json.Unmarshal(raw[start:i], &value); err != nil {
			result.Write(raw[start:i])
			continue
		}
		// Object keys describe the output contract and must remain stable.
		if i < len(raw) && raw[i] == ':' {
			result.Write(raw[start:i])
			continue
		}
		encoded, _ := json.Marshal(Redact(value))
		result.Write(encoded)
	}
	return result.String()
}
func (r *Reporter) Lifecycle(status string, data any) { r.Emit("lifecycle", status, data, nil, nil) }
func (r *Reporter) Finish(data any) int {
	code := ExitOK
	r.Emit("result", "ok", data, nil, &code)
	if r.writeErr != nil {
		return ExitFailure
	}
	return code
}
func (r *Reporter) Fail(code, message, hint string, exit int) int {
	r.Emit("error", "failed", nil, &Problem{code, message, hint}, &exit)
	return exit
}
func Redact(s string) string {
	secret := os.Getenv("NIB_DRIVER_TOKEN")
	if secret != "" {
		s = strings.ReplaceAll(s, secret, "[REDACTED]")
		encoded, _ := json.Marshal(secret)
		s = strings.ReplaceAll(s, string(encoded[1:len(encoded)-1]), "[REDACTED]")
	}
	return s
}

// WantsJSON respects known flag values and '--'. Unknown flags do not hide a later --json.
func WantsJSON(fs *flag.FlagSet, args []string) bool {
	mode := false
	if f := fs.Lookup("json"); f != nil {
		mode, _ = strconv.ParseBool(f.DefValue)
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name, value, eq := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name == "json" {
			if !eq {
				mode = true
			} else if v, e := strconv.ParseBool(value); e == nil {
				mode = v
			}
			continue
		}
		if f := fs.Lookup(name); f != nil && !eq {
			if b, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !b.IsBoolFlag() {
				i++
			}
		}
	}
	return mode
}

// Parse accepts flags before or after positionals (until '--'), while preserving flag values.
// All callers use ContinueOnError and route errors through Reporter, including help.
func Parse(fs *flag.FlagSet, args []string) error {
	var options, positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			positionals = append(positionals, a)
			continue
		}
		options = append(options, a)
		name, _, eq := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if f := fs.Lookup(name); f != nil && !eq {
			if b, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !b.IsBoolFlag() {
				if i+1 == len(args) {
					return fmt.Errorf("flag needs an argument: %s", a)
				}
				i++
				options = append(options, args[i])
			}
		}
	}
	return fs.Parse(append(append(options, "--"), positionals...))
}

// LogWriter redacts known diagnostic credentials, including secrets split across writes.
// Call Flush when the command ends to emit the final partial line.
type LogWriter struct {
	mu      sync.Mutex
	out     io.Writer
	tail    []byte
	secrets []string
}

func NewLogWriter(out io.Writer) *LogWriter {
	secret := os.Getenv("NIB_DRIVER_TOKEN")
	if secret == "" {
		return &LogWriter{out: out}
	}
	quoted := strconv.Quote(secret)
	return &LogWriter{out: out, secrets: []string{secret, quoted[1 : len(quoted)-1]}}
}
func (w *LogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tail = append(w.tail, p...)
	keep := 0
	for _, secret := range w.secrets {
		if len(secret)-1 > keep {
			keep = len(secret) - 1
		}
	}
	cut := len(w.tail) - keep
	if cut < 0 {
		cut = 0
	}
	for _, secret := range w.secrets {
		for start := 0; start < cut; {
			match := bytes.Index(w.tail[start:], []byte(secret))
			if match < 0 {
				break
			}
			match += start
			if match >= cut {
				break
			}
			if match+len(secret) > cut {
				cut = match
				break
			}
			start = match + len(secret)
		}
	}
	if cut > 0 {
		raw := string(w.tail[:cut])
		for _, secret := range w.secrets {
			raw = strings.ReplaceAll(raw, secret, "[REDACTED]")
		}
		if _, err := io.WriteString(w.out, raw); err != nil {
			return 0, err
		}
		w.tail = w.tail[cut:]
	}
	return len(p), nil
}
func (w *LogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := string(w.tail)
	for _, secret := range w.secrets {
		s = strings.ReplaceAll(s, secret, "[REDACTED]")
	}
	io.WriteString(w.out, s)
	w.tail = nil
}

// Failure carries a stable command error category across library layers.
type Failure struct {
	Code, Hint string
	Exit       int
	Cause      error
}

func (e *Failure) Error() string { return e.Cause.Error() }
func (e *Failure) Unwrap() error { return e.Cause }
func Fail(code, hint string, cause error) *Failure {
	return &Failure{Code: code, Hint: hint, Exit: ExitFailure, Cause: cause}
}
