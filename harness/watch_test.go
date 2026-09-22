package harness

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchChanges(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package x\n")

	changes, stop := WatchChanges(dir, 50*time.Millisecond)
	defer stop()

	waitNoEvent := func() {
		t.Helper()
		select {
		case <-changes:
			t.Fatal("不应有变更事件")
		case <-time.After(300 * time.Millisecond):
		}
	}
	waitEvent := func() {
		t.Helper()
		select {
		case <-changes:
		case <-time.After(2 * time.Second):
			t.Fatal("未等到变更事件")
		}
	}

	waitNoEvent() // 启动基准不触发

	write("a.go", "package x\n// 改了\n")
	waitEvent()

	write("b.go", "package x\n")
	waitEvent()

	if err := os.Remove(filepath.Join(dir, "b.go")); err != nil {
		t.Fatal(err)
	}
	waitEvent()
}

func TestGoFileSignatureSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".nib"), 0o755)
	os.WriteFile(filepath.Join(dir, ".nib", "x.go"), []byte("package x"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package x"), 0o644)

	sig, err := GoFileSignature(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 1 || sig["main.go"] == "" {
		t.Fatalf("签名应只含 main.go，实际 %v", sig)
	}
}
