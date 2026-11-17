package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"testing"
)

func TestParseFlagValuesAndTerminator(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("json", false, "")
	path := fs.String("out", "", "")
	if WantsJSON(fs, []string{"--out", "--json", "--", "--json"}) {
		t.Fatal("mistook a value for a JSON flag")
	}
	if err := Parse(fs, []string{"pkg", "--out", "value", "--json", "--", "-literal"}); err != nil {
		t.Fatal(err)
	}
	if *path != "value" || fs.NArg() != 2 || fs.Arg(1) != "-literal" {
		t.Fatal(fs.Args())
	}
}

func TestJSONRedactionPreservesStructure(t *testing.T) {
	for _, secret := range []string{`"`, `nib.cli/v1","command":"`, `secret"\\value`, "exit_code", "nib.cli/v1"} {
		t.Run(secret, func(t *testing.T) {
			t.Setenv("NIB_DRIVER_TOKEN", secret)
			var out bytes.Buffer
			r := Reporter{JSON: true, Command: "nib", Out: &out}
			r.Finish(map[string]string{"value": secret})
			var record Record
			if err := json.Unmarshal(out.Bytes(), &record); err != nil || record.Schema != Schema || record.Command != "nib" || record.ExitCode == nil {
				t.Fatalf("invalid record: %s (%v)", out.String(), err)
			}
		})
	}
}

func TestLogRedactionQuotedToken(t *testing.T) {
	secret := `1234567890123456"7890`
	t.Setenv("NIB_DRIVER_TOKEN", secret)
	var out bytes.Buffer
	w := NewLogWriter(&out)
	fmt.Fprintf(w, "msg=%q\n", secret)
	w.Flush()
	if strings.Contains(out.String(), `1234567890123456`) {
		t.Fatal(out.String())
	}
}

func TestHumanErrorRedaction(t *testing.T) {
	secret := "abcdefgh12345678"
	t.Setenv("NIB_DRIVER_TOKEN", secret)
	var out bytes.Buffer
	r := Reporter{Command: secret, Err: &out}
	r.Fail("test", secret, secret, ExitUsage)
	if strings.Contains(out.String(), secret) {
		t.Fatal(out.String())
	}
}
func TestLogRedactionAcrossWrites(t *testing.T) {
	t.Setenv("NIB_DRIVER_TOKEN", "secret-token-123456")
	var b bytes.Buffer
	w := NewLogWriter(&b)
	w.Write([]byte("prefix secret-tok"))
	w.Write([]byte("en-123456 suffix\n"))
	w.Flush()
	if b.String() != "prefix [REDACTED] suffix\n" {
		t.Fatal(b.String())
	}
}

func TestLogRedactionAcrossBufferBoundary(t *testing.T) {
	t.Setenv("NIB_DRIVER_TOKEN", "secret-token-123456")
	var b bytes.Buffer
	w := NewLogWriter(&b)
	w.Write([]byte("prefix secret-token-123456 suffix"))
	w.Flush()
	if b.String() != "prefix [REDACTED] suffix" {
		t.Fatal(b.String())
	}
}
