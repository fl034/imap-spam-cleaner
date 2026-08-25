package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
)

const defaultCodexTimeout = 180 * time.Second

const codexScoreSchema = `{
  "type": "object",
  "properties": {
    "score": { "type": "integer", "minimum": 0, "maximum": 100 }
  },
  "required": ["score"],
  "additionalProperties": false
}`

type Codex struct {
	command string
	model   string
	maxsize int
	timeout time.Duration
}

type codexScore struct {
	Score int `json:"score"`
}

func (p *Codex) Name() string {
	return "codex"
}

func (p *Codex) ValidateConfig(config map[string]string) error {
	p.command = config["command"]
	if p.command == "" {
		p.command = "codex"
	}
	p.model = config["model"]

	n, err := strconv.ParseInt(config["maxsize"], 10, 64)
	if err != nil || n < 1 {
		return errors.New("codex maxsize must be a positive integer")
	}
	p.maxsize = int(n)

	p.timeout = defaultCodexTimeout
	if config["timeout"] != "" {
		if p.timeout, err = time.ParseDuration(config["timeout"]); err != nil || p.timeout <= 0 {
			return errors.New("codex timeout must be a positive duration (eg. 30s or 2m)")
		}
	}

	return nil
}

func (p *Codex) Init(config map[string]string) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}
	if _, err := exec.LookPath(p.command); err != nil {
		return fmt.Errorf("codex command %q not found: %w", p.command, err)
	}
	return nil
}

func (p *Codex) Analyze(msg imap.Message) (int, error) {
	schema, err := os.CreateTemp("", "imap-spam-cleaner-codex-schema-*.json")
	if err != nil {
		return 0, fmt.Errorf("create codex output schema: %w", err)
	}
	schemaPath := schema.Name()
	defer os.Remove(schemaPath)

	if _, err = schema.WriteString(codexScoreSchema); err != nil {
		schema.Close()
		return 0, fmt.Errorf("write codex output schema: %w", err)
	}
	if err = schema.Close(); err != nil {
		return 0, fmt.Errorf("close codex output schema: %w", err)
	}

	args := []string{
		"exec",
		"--ephemeral",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ignore-rules",
		"--sandbox", "read-only",
		"--color", "never",
		"--output-schema", schemaPath,
	}
	if p.model != "" {
		args = append(args, "--model", p.model)
	}
	args = append(args, "-")

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(codexAnalysisPrompt(msg, p.maxsize))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err = cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, fmt.Errorf("codex command timed out after %s", p.timeout)
		}
		return 0, fmt.Errorf("codex command failed: %w", err)
	}

	var result codexScore
	if err = json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return 0, fmt.Errorf("invalid codex response: %w", err)
	}
	if result.Score < 0 || result.Score > 100 {
		return 0, fmt.Errorf("codex score must be between 0 and 100, got %d", result.Score)
	}

	return result.Score, nil
}
