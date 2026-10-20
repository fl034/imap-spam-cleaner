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

const defaultClaudeTimeout = 180 * time.Second

type Claude struct {
	command string
	model   string
	maxsize int
	timeout time.Duration
}

type claudeResult struct {
	IsError          bool        `json:"is_error"`
	Result           string      `json:"result"`
	StructuredOutput *codexScore `json:"structured_output"`
}

func (p *Claude) Name() string {
	return "claude"
}

func (p *Claude) ValidateConfig(config map[string]string) error {
	p.command = config["command"]
	if p.command == "" {
		p.command = "claude"
	}
	p.model = config["model"]

	n, err := strconv.ParseInt(config["maxsize"], 10, 64)
	if err != nil || n < 1 {
		return errors.New("claude maxsize must be a positive integer")
	}
	p.maxsize = int(n)

	p.timeout = defaultClaudeTimeout
	if config["timeout"] != "" {
		if p.timeout, err = time.ParseDuration(config["timeout"]); err != nil || p.timeout <= 0 {
			return errors.New("claude timeout must be a positive duration (eg. 30s or 2m)")
		}
	}

	return nil
}

func (p *Claude) Init(config map[string]string) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}
	if _, err := exec.LookPath(p.command); err != nil {
		return fmt.Errorf("claude command %q not found: %w", p.command, err)
	}
	return nil
}

func (p *Claude) Analyze(msg imap.Message) (int, error) {
	args := []string{
		"--print",
		"--output-format", "json",
		"--json-schema", codexScoreSchema,
		"--tools", "",
		"--strict-mcp-config",
		"--setting-sources", "",
		"--no-session-persistence",
	}
	if p.model != "" {
		args = append(args, "--model", p.model)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(codexAnalysisPrompt(msg, p.maxsize))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("claude command timed out after %s", p.timeout)
	}

	var result claudeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		if runErr != nil {
			return 0, fmt.Errorf("claude command failed: %w: %s", runErr, strings.TrimSpace(stderr.String()))
		}
		return 0, fmt.Errorf("invalid claude response: %w", err)
	}
	if result.IsError || runErr != nil {
		return 0, fmt.Errorf("claude command failed: %s", result.Result)
	}
	if result.StructuredOutput == nil {
		return 0, fmt.Errorf("claude response has no structured output: %s", result.Result)
	}
	if score := result.StructuredOutput.Score; score < 0 || score > 100 {
		return 0, fmt.Errorf("claude score must be between 0 and 100, got %d", score)
	}

	return result.StructuredOutput.Score, nil
}
