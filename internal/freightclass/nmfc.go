package freightclass

import (
	"context"
	"strings"
)

// NMFCItem is a curated National Motor Freight Classification entry. When
// FixedClass is non-nil the commodity carries that class regardless of density;
// otherwise the item is density-rated and only used to identify the commodity.
type NMFCItem struct {
	Code        string
	Description string
	FixedClass  *Class
	Keywords    []string
}

func fixed(c Class) *Class { return &c }

// NMFCMatcher resolves a free-text goods description to an NMFC catalog item.
// It is the AI integration seam: the POC ships KeywordMatcher (deterministic,
// offline), but an LLM/semantic matcher can implement this same interface and
// be swapped in via Matcher without touching the recommend pipeline — see
// nmfc_llm.go.
type NMFCMatcher interface {
	Match(ctx context.Context, description string) (item NMFCItem, ok bool, err error)
}

// Matcher is the active NMFC matcher. Swap it (e.g. to an LLMMatcher) to change
// classification behaviour process-wide.
var Matcher NMFCMatcher = KeywordMatcher{}

// MatchNMFC is a convenience wrapper over the active Matcher.
func MatchNMFC(description string) (NMFCItem, bool) {
	item, ok, err := Matcher.Match(context.Background(), description)
	if err != nil {
		return NMFCItem{}, false
	}
	return item, ok
}

// nmfcCatalog is a small, illustrative sample set for the POC. The full NMFC
// book is licensed; these codes/classes are representative demo data only.
var nmfcCatalog = []NMFCItem{
	// Fixed-class commodities (class overrides measured density).
	{Code: "86150", Description: "Table tennis / ping pong balls", FixedClass: fixed(500), Keywords: []string{"ping pong", "ping-pong", "table tennis"}},
	{Code: "32050", Description: "Canoes or kayaks", FixedClass: fixed(400), Keywords: []string{"canoe", "kayak"}},
	{Code: "124530", Description: "Mattresses", FixedClass: fixed(150), Keywords: []string{"mattress", "mattresses"}},
	{Code: "156300", Description: "Foam / expanded plastic, packing peanuts", FixedClass: fixed(300), Keywords: []string{"foam", "styrofoam", "packing peanuts"}},

	// Density-rated commodities (matcher names the item; class stays density-based).
	{Code: "61000", Description: "Machinery or machine parts", Keywords: []string{"machine", "machinery", "motor", "engine", "pump"}},
	{Code: "19080", Description: "Books, printed matter", Keywords: []string{"book", "books", "printed", "paper"}},
	{Code: "49880", Description: "Clothing or apparel", Keywords: []string{"clothing", "apparel", "garment", "textile", "clothes"}},
	{Code: "99080", Description: "Automotive parts", Keywords: []string{"automotive", "car part", "auto part", "brake", "tire", "tires"}},
	{Code: "133300", Description: "Metal fasteners (nuts, bolts, screws)", Keywords: []string{"nuts", "bolts", "fastener", "screws", "hardware"}},
	{Code: "63700", Description: "Electronics (televisions, monitors)", Keywords: []string{"television", "monitor", "electronic", "computer"}},
}

// KeywordMatcher scores the curated catalog by keyword occurrences. Offline,
// deterministic, and dependency-free — it powers the demo and serves as the
// fallback for the LLM matcher.
type KeywordMatcher struct{}

// Match returns the best-matching catalog item for a free-text description,
// scored by keyword occurrences. ok is false when nothing matches.
func (KeywordMatcher) Match(_ context.Context, description string) (NMFCItem, bool, error) {
	d := strings.ToLower(strings.TrimSpace(description))
	if d == "" {
		return NMFCItem{}, false, nil
	}

	best := -1
	bestScore := 0
	for i, item := range nmfcCatalog {
		score := 0
		for _, kw := range item.Keywords {
			if strings.Contains(d, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = i
		}
	}

	if best < 0 {
		return NMFCItem{}, false, nil
	}
	return nmfcCatalog[best], true, nil
}
