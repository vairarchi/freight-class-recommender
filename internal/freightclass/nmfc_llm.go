package freightclass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/pitneybowes/innovation/pbgolib/core/envvar"
	"gitlab.com/pitneybowes/innovation/pbgolib/core/logging"
)

// confidenceThreshold is the minimum model confidence to accept an LLM match;
// below it we fall back to the deterministic keyword matcher.
const confidenceThreshold = 0.6

// httpDoer is the minimal HTTP surface LLMMatcher needs, so tests can inject a
// mock transport.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// LLMMatcher maps a free-text goods description to an NMFC item using AWS
// Bedrock (Anthropic Claude). It grounds the model with the curated catalog,
// confidence-gates the reply, and falls back to the deterministic keyword
// matcher on low confidence or any error, so the experience never breaks.
//
// It is NOT wired in by default — enable it in main.go:
//
//	freightclass.Matcher = freightclass.NewLLMMatcher()
type LLMMatcher struct {
	APIToken string
	Region   string
	ModelID  string
	client   httpDoer
	fallback NMFCMatcher
}

// NewLLMMatcher reads Bedrock config from the environment and returns a matcher
// backed by the keyword matcher as a fallback.
func NewLLMMatcher() *LLMMatcher {
	token, _ := envvar.GetString("APP_BEDROCK_API_TOKEN")
	region, _ := envvar.GetString("APP_BEDROCK_REGION")
	modelID, _ := envvar.GetString("APP_BEDROCK_MODEL_ID")
	return &LLMMatcher{
		APIToken: token,
		Region:   region,
		ModelID:  modelID,
		client:   &http.Client{Timeout: 20 * time.Second},
		fallback: KeywordMatcher{},
	}
}

// Match resolves the description via Bedrock, falling back to the keyword
// matcher when unconfigured, low-confidence, or on error.
func (m *LLMMatcher) Match(ctx context.Context, description string) (NMFCItem, bool, error) {
	d := strings.TrimSpace(description)
	if d == "" {
		return NMFCItem{}, false, nil
	}
	if m.APIToken == "" || m.Region == "" || m.ModelID == "" {
		logging.Debug(ctx, "LLM matcher not configured; using keyword fallback")
		return m.fallback.Match(ctx, d)
	}

	item, ok, err := m.matchViaBedrock(ctx, d)
	if err != nil {
		logging.WithError(ctx, "LLM matcher failed; falling back to keyword", err)
		return m.fallback.Match(ctx, d)
	}
	if !ok {
		return m.fallback.Match(ctx, d)
	}
	return item, true, nil
}

// bedrock request/response shapes (Anthropic Messages API on Bedrock).
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type bedrockClaudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	System           string          `json:"system,omitempty"`
	Messages         []claudeMessage `json:"messages"`
}

type bedrockClaudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// llmNMFCReply is the structured JSON we instruct the model to return.
type llmNMFCReply struct {
	Code        string   `json:"code"`
	Description string   `json:"description"`
	FixedClass  *float64 `json:"fixedClass"`
	Confidence  float64  `json:"confidence"`
}

func (m *LLMMatcher) matchViaBedrock(ctx context.Context, description string) (NMFCItem, bool, error) {
	system, user := buildPrompt(description)
	reqBody := bedrockClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        256,
		System:           system,
		Messages:         []claudeMessage{{Role: "user", Content: user}},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return NMFCItem{}, false, err
	}

	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
		m.Region, url.PathEscape(m.ModelID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return NMFCItem{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return NMFCItem{}, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NMFCItem{}, false, err
	}
	if resp.StatusCode != http.StatusOK {
		return NMFCItem{}, false, fmt.Errorf("bedrock returned status %d: %s", resp.StatusCode, string(body))
	}

	var br bedrockClaudeResponse
	if err := json.Unmarshal(body, &br); err != nil {
		return NMFCItem{}, false, err
	}
	if len(br.Content) == 0 {
		return NMFCItem{}, false, fmt.Errorf("bedrock response had no content")
	}

	reply, err := parseNMFCReply(br.Content[0].Text)
	if err != nil {
		return NMFCItem{}, false, err
	}
	if reply.Confidence < confidenceThreshold || reply.Code == "" {
		return NMFCItem{}, false, nil
	}

	// Prefer the curated catalog entry so downstream fields stay canonical.
	if item, ok := catalogByCode(reply.Code); ok {
		return item, true, nil
	}
	var fc *Class
	if reply.FixedClass != nil {
		c := Class(*reply.FixedClass)
		fc = &c
	}
	return NMFCItem{Code: reply.Code, Description: reply.Description, FixedClass: fc}, true, nil
}

// buildPrompt grounds the model with the curated catalog and asks for strict JSON.
func buildPrompt(description string) (system, user string) {
	var b strings.Builder
	for _, it := range nmfcCatalog {
		fmt.Fprintf(&b, "- %s: %s", it.Code, it.Description)
		if it.FixedClass != nil {
			fmt.Fprintf(&b, " (fixed class %s)", formatClass(*it.FixedClass))
		}
		b.WriteString("\n")
	}
	system = "You are an NMFC freight classification expert. Map the shipment goods " +
		"description to the single best NMFC item from the candidate list. If the chosen " +
		"item is fixed-class, include its class; otherwise set fixedClass to null. Respond " +
		"ONLY with compact JSON and nothing else: " +
		`{"code":"<nmfc code>","description":"<item>","fixedClass":<number or null>,"confidence":<0..1>}`
	user = "Candidates:\n" + b.String() + "\nGoods description: " + description
	return system, user
}

// parseNMFCReply extracts the first JSON object from the model text and decodes it.
func parseNMFCReply(text string) (llmNMFCReply, error) {
	var reply llmNMFCReply
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return reply, fmt.Errorf("no JSON object in model reply")
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &reply); err != nil {
		return reply, err
	}
	return reply, nil
}

func catalogByCode(code string) (NMFCItem, bool) {
	for _, it := range nmfcCatalog {
		if it.Code == code {
			return it, true
		}
	}
	return NMFCItem{}, false
}
