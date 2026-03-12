package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
)

func TestNVIDIAInitUsesConfiguredTimeout(t *testing.T) {
	p := &NVIDIA{}
	err := p.Init(map[string]string{
		"apikey":  "test-key",
		"model":   "test-model",
		"maxsize": "1000",
		"timeout": "125s",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if p.client == nil {
		t.Fatal("expected HTTP client to be initialized")
	}

	if p.client.Timeout != 125*time.Second {
		t.Fatalf("expected timeout to be 125s, got %s", p.client.Timeout)
	}
	if p.timeout != 125*time.Second {
		t.Fatalf("expected provider timeout to be 125s, got %s", p.timeout)
	}
}

func TestNVIDIAValidateConfigRejectsInvalidTimeout(t *testing.T) {
	p := &NVIDIA{}
	err := p.ValidateConfig(map[string]string{
		"apikey":  "test-key",
		"model":   "test-model",
		"maxsize": "1000",
		"timeout": "never",
	})
	if err == nil {
		t.Fatal("expected invalid timeout to fail validation")
	}
}

func TestNVIDIAAnalyzeHonorsConfiguredTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(75 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"42"}}]}`))
	}))
	defer server.Close()

	p := &NVIDIA{}
	err := p.Init(map[string]string{
		"apikey":  "test-key",
		"url":     server.URL,
		"model":   "test-model",
		"maxsize": "1000",
		"timeout": "200ms",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	score, err := p.Analyze(imap.Message{Contents: []string{"hello"}})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if score != 42 {
		t.Fatalf("expected score 42, got %d", score)
	}
}