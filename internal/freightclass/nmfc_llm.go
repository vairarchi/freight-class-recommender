package freightclass

import (
	"context"

	"gitlab.com/pitneybowes/innovation/pbgolib/core/envvar"
)

// LLMMatcher is the explicit integration seam for the hackathon's AI use case:
// given a free-text goods description, an LLM maps it to the correct NMFC item
// (including fixed-class commodities that density alone cannot determine).
//
// It is NOT wired in by default — the demo uses the deterministic KeywordMatcher.
// To enable it, set the active matcher in main.go:
//
//	freightclass.Matcher = freightclass.NewLLMMatcher()
//
// TODO(llm): implement Match with a real LLM call:
//  1. Build a prompt from `description` plus NMFC candidates (few-shot, or
//     retrieval/embeddings over the licensed NMFC book) to ground the model.
//  2. Call Bedrock through pbgolib restclient (no external SDKs) at the given
//     region/model, passing ctx and the APIToken as a bearer header; request
//     structured JSON {code, description, fixedClass, confidence}.
//  3. Parse the reply into an NMFCItem and apply a confidence threshold.
//  4. Fall back to the keyword matcher on low confidence or error so the
//     experience never breaks (see fallback below).
type LLMMatcher struct {
	APIToken string
	Region   string
	ModelID  string
	fallback NMFCMatcher
}

// NewLLMMatcher reads Bedrock config from the environment and returns a matcher
// that falls back to the deterministic keyword matcher until the LLM call is
// implemented.
func NewLLMMatcher() *LLMMatcher {
	token, _ := envvar.GetString("APP_BEDROCK_API_TOKEN")
	region, _ := envvar.GetString("APP_BEDROCK_REGION")
	modelID, _ := envvar.GetString("APP_BEDROCK_MODEL_ID")
	return &LLMMatcher{
		APIToken: token,
		Region:   region,
		ModelID:  modelID,
		fallback: KeywordMatcher{},
	}
}

// Match currently delegates to the keyword fallback. Replace the body with the
// Bedrock call described in the TODO above; keep the fallback for resilience.
func (m *LLMMatcher) Match(ctx context.Context, description string) (NMFCItem, bool, error) {
	// TODO(llm): call Bedrock here, then confidence-gate and fall back.
	return m.fallback.Match(ctx, description)
}
