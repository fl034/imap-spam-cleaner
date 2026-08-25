package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
)

func TestCodexValidateConfig(t *testing.T) {
	p := &Codex{}
	err := p.ValidateConfig(map[string]string{
		"maxsize": "1000",
		"timeout": "45s",
	})
	if err != nil {
		t.Fatalf("ValidateConfig returned error: %v", err)
	}
	if p.command != "codex" {
		t.Fatalf("expected default command codex, got %q", p.command)
	}
	if p.timeout != 45*time.Second {
		t.Fatalf("expected timeout 45s, got %s", p.timeout)
	}
}

func TestCodexAnalyzeUsesStructuredOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script")
	}

	dir := t.TempDir()
	command := filepath.Join(dir, "fake-codex")
	script := `#!/bin/sh
schema=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-schema" ]; then
    schema="$2"
    shift 2
    continue
  fi
  shift
done
test -s "$schema" || exit 2
prompt=$(cat)
case "$prompt" in
  *"Subject: Limited offer"*) ;;
  *) exit 3 ;;
esac
printf '{"score":87}\n'
`
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake codex command: %v", err)
	}

	p := &Codex{}
	if err := p.Init(map[string]string{
		"command": command,
		"model":   "test-model",
		"maxsize": "1000",
		"timeout": "2s",
	}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	score, err := p.Analyze(imap.Message{Subject: "Limited offer", Contents: []string{"Buy now"}})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if score != 87 {
		t.Fatalf("expected score 87, got %d", score)
	}
}

func TestCodexAnalyzeReportsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script")
	}

	command := filepath.Join(t.TempDir(), "slow-codex")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nsleep 1\n"), 0o700); err != nil {
		t.Fatalf("write fake codex command: %v", err)
	}

	p := &Codex{}
	if err := p.Init(map[string]string{
		"command": command,
		"maxsize": "1000",
		"timeout": "10ms",
	}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	_, err := p.Analyze(imap.Message{Contents: []string{"hello"}})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}
