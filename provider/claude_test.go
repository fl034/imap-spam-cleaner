package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
)

func fakeClaude(t *testing.T, script string) *Claude {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script")
	}
	command := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake claude command: %v", err)
	}
	p := &Claude{}
	if err := p.Init(map[string]string{"command": command, "maxsize": "1000", "timeout": "2s"}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	return p
}

func TestClaudeAnalyzeUsesStructuredOutput(t *testing.T) {
	p := fakeClaude(t, `#!/bin/sh
case "$*" in *"--json-schema"*) ;; *) exit 2 ;; esac
case "$(cat)" in *"Subject: Limited offer"*) ;; *) exit 3 ;; esac
printf '{"is_error":false,"result":"{\\"score\\":87}","structured_output":{"score":87}}\n'
`)
	score, err := p.Analyze(imap.Message{Subject: "Limited offer", Contents: []string{"Buy now"}})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if score != 87 {
		t.Fatalf("expected score 87, got %d", score)
	}
}

func TestClaudeAnalyzeReportsCLIError(t *testing.T) {
	p := fakeClaude(t, `#!/bin/sh
printf '{"is_error":true,"result":"Not logged in · Please run /login"}\n'
exit 1
`)
	_, err := p.Analyze(imap.Message{Contents: []string{"hello"}})
	if err == nil || !strings.Contains(err.Error(), "Not logged in") {
		t.Fatalf("expected login error, got %v", err)
	}
}
