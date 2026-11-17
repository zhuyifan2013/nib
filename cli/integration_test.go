package cli_test

import (
	"bytes"
	"encoding/json"
	"nib.dev/nib/cli"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandResults(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(file))
	temp := t.TempDir()
	bins := map[string]string{}
	for _, name := range []string{"nib", "nibgen"} {
		path := filepath.Join(temp, name)
		cmd := exec.Command("go", "build", "-o", path, "./cmd/"+name)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, output)
		}
		bins[name] = path
	}
	fixture := filepath.Join(root, "binding/testdata/example")
	noService := filepath.Join(temp, "empty")
	os.Mkdir(noService, 0700)
	os.WriteFile(filepath.Join(noService, "empty.go"), []byte("package empty"), 0600)
	broken := filepath.Join(temp, "broken")
	os.Mkdir(broken, 0700)
	os.WriteFile(filepath.Join(broken, "bad.go"), []byte("package broken\nfunc syntax("), 0600)
	blocked := filepath.Join(temp, "not-a-directory")
	os.WriteFile(blocked, []byte("x"), 0600)
	cases := []struct {
		name, bin string
		args      []string
		exit      int
		code      string
	}{
		{"unknown", "nib", []string{"missing", "--json"}, 2, "cli/unknown_command"},
		{"global_no_command", "nib", []string{"--json"}, 2, "cli/invalid_args"},
		{"invalid_flag", "nib", []string{"dev", "--bad", "--json"}, 2, "cli/invalid_flag"},
		{"invalid_duration", "nib", []string{"dev", "--json", "--interval", "wat", "./cmd/demo"}, 2, "cli/invalid_flag"},
		{"missing_pkg", "nib", []string{"--json", "dev"}, 2, "cli/invalid_args"},
		{"missing_sock", "nib", []string{"shell", "--json"}, 2, "cli/invalid_args"},
		{"missing_value", "nib", []string{"probe", "--json", "--sock"}, 2, "cli/invalid_flag"},
		{"conflict", "nib", []string{"dev", "./cmd/demo", "--json", "--events", "-"}, 2, "cli/output_conflict"},
		{"build_failure", "nib", []string{"dev", "./missing-package-json-test", "--json"}, 1, "dev/build"},
		{"probe_failure", "nib", []string{"probe", "--json", "--sock", filepath.Join(temp, "missing.sock")}, 1, "probe/connect"},
		{"gen_load", "nibgen", []string{broken, "--json"}, 1, "nibgen/load"},
		{"gen_empty", "nibgen", []string{"--json", noService}, 1, "nibgen/no_services"},
		{"gen_write", "nibgen", []string{fixture, "--out", blocked, "--json"}, 1, "nibgen/generate"},
		{"gen_flag", "nibgen", []string{"--bad", "--json"}, 2, "cli/invalid_flag"},
		{"gen_extra", "nibgen", []string{fixture, fixture, "--json"}, 2, "cli/invalid_args"},
		{"gen_success", "nibgen", []string{fixture, "--out", filepath.Join(temp, "bindings"), "--json"}, 0, ""},
		{"help", "nib", []string{"--json", "--help"}, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(bins[c.bin], c.args...)
			cmd.Dir = root
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			exit := 0
			if e, ok := err.(*exec.ExitError); ok {
				exit = e.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if exit != c.exit {
				t.Fatalf("exit %d want%d stderr=%s stdout=%s", exit, c.exit, stderr.String(), out.String())
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			var last cli.Record
			for _, line := range lines {
				var record cli.Record
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("non-JSON stdout: %q stderr=%s", line, stderr.String())
				}
				if record.Schema != cli.Schema {
					t.Fatal(record)
				}
				last = record
			}
			if last.ExitCode == nil || *last.ExitCode != c.exit {
				t.Fatal("missing final exit", out.String())
			}
			if c.code != "" && (last.Error == nil || last.Error.Code != c.code || last.Error.Hint == "") {
				t.Fatal("invalid error", out.String())
			}
		})
	}
	// Redaction must apply even when flag parser includes a diagnostic credential in its error.
	secret := "test-private-credential-123"
	cmd := exec.Command(bins["nib"], "dev", "--json", "--driver-timeout", secret, "./cmd/demo")
	cmd.Env = append(os.Environ(), "NIB_DRIVER_TOKEN="+secret)
	out, err := cmd.CombinedOutput()
	if err == nil || bytes.Contains(out, []byte(secret)) {
		t.Fatal("credential exposed", string(out))
	}
	cmd = exec.Command(bins["nib"], "doctor", "--json")
	out, err = cmd.Output()
	var record cli.Record
	if json.Unmarshal(out, &record) != nil || record.Data == nil {
		t.Fatal("doctor output", string(out), err)
	}
}
