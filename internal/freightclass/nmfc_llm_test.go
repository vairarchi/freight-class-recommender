package freightclass

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockDoer returns a canned response/error for any request.
type mockDoer struct {
	resp *http.Response
	err  error
}

func (m mockDoer) Do(*http.Request) (*http.Response, error) { return m.resp, m.err }

// bedrockResp builds a Bedrock Claude response whose single text block is innerText.
func bedrockResp(status int, innerText string) *http.Response {
	payload := `{"content":[{"type":"text","text":` + jsonQuote(innerText) + `}]}`
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(payload)),
		Header:     make(http.Header),
	}
}

// jsonQuote escapes a string as a JSON string literal.
func jsonQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func newTestLLMMatcher(d httpDoer) *LLMMatcher {
	return &LLMMatcher{
		APIToken: "test-token",
		Region:   "us-east-1",
		ModelID:  "anthropic.claude-3-5-sonnet-20240620-v1:0",
		client:   d,
		fallback: KeywordMatcher{},
	}
}

func TestLLMMatcherSuccess(t *testing.T) {
	inner := `{"code":"86150","description":"Table tennis / ping pong balls","fixedClass":500,"confidence":0.95}`
	m := newTestLLMMatcher(mockDoer{resp: bedrockResp(http.StatusOK, inner)})

	item, ok, err := m.Match(context.Background(), "a crate of ping pong balls")
	if err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	if item.Code != "86150" || item.FixedClass == nil || *item.FixedClass != 500 {
		t.Errorf("got %+v, want catalog item 86150 fixed 500", item)
	}
}

func TestLLMMatcherLowConfidenceFallsBack(t *testing.T) {
	// Low confidence -> ignore LLM, fall back to keyword, which matches "machine".
	inner := `{"code":"86150","description":"ping pong","fixedClass":500,"confidence":0.2}`
	m := newTestLLMMatcher(mockDoer{resp: bedrockResp(http.StatusOK, inner)})

	item, ok, err := m.Match(context.Background(), "assorted machine parts")
	if err != nil || !ok {
		t.Fatalf("expected keyword fallback match, got ok=%v err=%v", ok, err)
	}
	if item.Code != "61000" {
		t.Errorf("fallback got %q, want keyword match 61000", item.Code)
	}
}

func TestLLMMatcherHTTPErrorFallsBack(t *testing.T) {
	m := newTestLLMMatcher(mockDoer{resp: bedrockResp(http.StatusInternalServerError, "oops")})

	item, ok, _ := m.Match(context.Background(), "mattress")
	if !ok || item.Code != "124530" {
		t.Errorf("on HTTP error should fall back to keyword (124530), got ok=%v code=%q", ok, item.Code)
	}
}

func TestLLMMatcherUnconfiguredFallsBack(t *testing.T) {
	m := &LLMMatcher{fallback: KeywordMatcher{}} // no token/region/model
	item, ok, _ := m.Match(context.Background(), "books and printed matter")
	if !ok || item.Code != "19080" {
		t.Errorf("unconfigured should use keyword (19080), got ok=%v code=%q", ok, item.Code)
	}
}

func TestParseNMFCReplyExtractsJSON(t *testing.T) {
	reply, err := parseNMFCReply("Here you go:\n{\"code\":\"61000\",\"confidence\":0.8}\nThanks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply.Code != "61000" || reply.Confidence != 0.8 {
		t.Errorf("got %+v, want code 61000 conf 0.8", reply)
	}
}
