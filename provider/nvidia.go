package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
)

const defaultNVIDIAURL = "https://integrate.api.nvidia.com/v1/chat/completions"
const defaultNVIDIAModel = "moonshotai/kimi-k2.5"
const defaultNVIDIATimeout = 180 * time.Second

type NVIDIA struct {
	client      *http.Client
	url         string
	apikey      string
	model       string
	maxsize     int
	maxTokens   int
	temperature float64
	topP        float64
	thinking    bool
	timeout     time.Duration
}

type nvidiaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type nvidiaChatRequest struct {
	Model              string               `json:"model"`
	Messages           []nvidiaChatMessage  `json:"messages"`
	MaxTokens          int                  `json:"max_tokens"`
	Temperature        float64              `json:"temperature"`
	TopP               float64              `json:"top_p"`
	Stream             bool                 `json:"stream"`
	ChatTemplateKwargs map[string]bool      `json:"chat_template_kwargs,omitempty"`
}

type nvidiaChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *NVIDIA) Name() string {
	return "nvidia"
}

func (p *NVIDIA) ValidateConfig(config map[string]string) error {
	if config["apikey"] == "" {
		return errors.New("nvidia apikey is required")
	}
	p.apikey = config["apikey"]

	p.url = defaultNVIDIAURL
	if config["url"] != "" {
		p.url = config["url"]
	}

	p.model = defaultNVIDIAModel
	if config["model"] != "" {
		p.model = config["model"]
	}

	n, err := strconv.ParseInt(config["maxsize"], 10, 64)
	if err != nil || n < 1 {
		return errors.New("nvidia maxsize must be a positive integer")
	}
	p.maxsize = int(n)

	p.maxTokens = 16384
	if config["maxtokens"] != "" {
		n, err = strconv.ParseInt(config["maxtokens"], 10, 64)
		if err != nil || n < 1 {
			return errors.New("nvidia maxtokens must be a positive integer")
		}
		p.maxTokens = int(n)
	}

	p.temperature = 1.0
	if config["temperature"] != "" {
		f, err := strconv.ParseFloat(config["temperature"], 64)
		if err != nil {
			return errors.New("nvidia temperature must be a number")
		}
		p.temperature = f
	}

	p.topP = 1.0
	if config["topp"] != "" {
		f, err := strconv.ParseFloat(config["topp"], 64)
		if err != nil {
			return errors.New("nvidia topp must be a number")
		}
		p.topP = f
	}

	p.thinking = true
	if config["thinking"] != "" {
		b, err := strconv.ParseBool(config["thinking"])
		if err != nil {
			return errors.New("nvidia thinking must be true or false")
		}
		p.thinking = b
	}

	p.timeout = defaultNVIDIATimeout
	if config["timeout"] != "" {
		if to, err := time.ParseDuration(config["timeout"]); err == nil && to > 0 {
			p.timeout = to
		} else {
			t, err := strconv.ParseFloat(config["timeout"], 64)
			if err != nil || t <= 0 {
				return errors.New("nvidia timeout must be a duration (eg. 30s, 2m) or a positive number of seconds")
			}
			p.timeout = time.Duration(t * float64(time.Second))
		}
	}

	return nil
}

func (p *NVIDIA) Init(config map[string]string) error {
	if err := p.ValidateConfig(config); err != nil {
		return err
	}

	p.client = &http.Client{Timeout: p.timeout}
	return nil
}

func (p *NVIDIA) Analyze(msg imap.Message) (int, error) {
	reqBody := nvidiaChatRequest{
		Model: p.model,
		Messages: []nvidiaChatMessage{
			{
				Role:    "user",
				Content: analysisPrompt(msg, p.maxsize),
			},
		},
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
		TopP:        p.topP,
		Stream:      false,
		ChatTemplateKwargs: map[string]bool{
			"thinking": p.thinking,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest(http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apikey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var chatResp nvidiaChatResponse
	if err = json.Unmarshal(respBody, &chatResp); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return 0, fmt.Errorf("nvidia request failed with status %s: %s", resp.Status, string(respBody))
		}
		return 0, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		if chatResp.Error != nil && chatResp.Error.Message != "" {
			return 0, fmt.Errorf("nvidia request failed with status %s: %s", resp.Status, chatResp.Error.Message)
		}
		return 0, fmt.Errorf("nvidia request failed with status %s", resp.Status)
	}

	if len(chatResp.Choices) == 0 {
		return 0, errors.New("nvidia response did not contain any choices")
	}

	return parseScore(chatResp.Choices[0].Message.Content)
}