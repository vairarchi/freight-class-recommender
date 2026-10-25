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

// nmfcCatalog is a curated, illustrative sample set for the POC. The full NMFC
// book is licensed; these codes/classes are representative demo data only.
// Fixed-class items override measured density; the rest are density-rated and
// only identify the commodity. At this size the whole catalog is sent to the
// LLM as grounding; beyond ~50–100 items switch to embeddings/RAG retrieval.
var nmfcCatalog = []NMFCItem{
	// --- Fixed-class commodities (class overrides measured density) ---
	{Code: "86150", Description: "Table tennis / ping pong balls", FixedClass: fixed(500), Keywords: []string{"ping pong", "ping-pong", "table tennis"}},
	{Code: "32050", Description: "Canoes or kayaks", FixedClass: fixed(400), Keywords: []string{"canoe", "kayak"}},
	{Code: "124530", Description: "Mattresses", FixedClass: fixed(150), Keywords: []string{"mattress", "mattresses"}},
	{Code: "156300", Description: "Foam / expanded plastic, packing peanuts", FixedClass: fixed(300), Keywords: []string{"foam", "styrofoam", "packing peanuts"}},
	{Code: "110400", Description: "Lamp shades", FixedClass: fixed(500), Keywords: []string{"lamp shade", "lampshade", "lamp shades"}},
	{Code: "48580", Description: "Artificial flowers or plants", FixedClass: fixed(300), Keywords: []string{"artificial flower", "artificial plant", "fake plant", "silk flower"}},
	{Code: "13050", Description: "Balloons, toy or decorative", FixedClass: fixed(400), Keywords: []string{"balloon", "balloons"}},
	{Code: "125670", Description: "Pillows or cushions", FixedClass: fixed(175), Keywords: []string{"pillow", "pillows", "cushion", "cushions"}},
	{Code: "60100", Description: "Kayak paddles / canoe paddles", FixedClass: fixed(150), Keywords: []string{"paddle", "paddles", "oar", "oars"}},
	{Code: "22400", Description: "Bicycles, assembled", FixedClass: fixed(150), Keywords: []string{"bicycle", "bike", "bicycles"}},

	// --- Density-rated commodities (matcher names the item; class from density) ---
	{Code: "61000", Description: "Machinery or machine parts", Keywords: []string{"machine", "machinery", "motor", "engine", "pump"}},
	{Code: "19080", Description: "Books, printed matter", Keywords: []string{"book", "books", "printed", "paper"}},
	{Code: "49880", Description: "Clothing or apparel", Keywords: []string{"clothing", "apparel", "garment", "textile", "clothes"}},
	{Code: "99080", Description: "Automotive parts", Keywords: []string{"automotive", "car part", "auto part", "brake", "tire", "tires", "alternator"}},
	{Code: "133300", Description: "Metal fasteners (nuts, bolts, screws)", Keywords: []string{"nuts", "bolts", "fastener", "screws", "hardware"}},
	{Code: "63700", Description: "Electronics (televisions, monitors)", Keywords: []string{"television", "monitor", "electronic", "computer", "laptop"}},
	{Code: "70720", Description: "Furniture, wooden", Keywords: []string{"furniture", "chair", "table", "desk", "cabinet", "dresser"}},
	{Code: "73380", Description: "Canned goods / packaged foodstuffs", Keywords: []string{"canned", "food", "grocery", "beverage", "soda", "bottled water"}},
	{Code: "156600", Description: "Plastic containers and bottles", Keywords: []string{"plastic bottle", "container", "jug", "bucket", "plastic jar"}},
	{Code: "110260", Description: "Hand tools", Keywords: []string{"tool", "tools", "hammer", "wrench", "drill", "pliers"}},
	{Code: "50700", Description: "Steel pipe or tubing", Keywords: []string{"pipe", "tubing", "conduit", "steel pipe"}},
	{Code: "85300", Description: "Lumber / wood boards", Keywords: []string{"lumber", "wood", "plywood", "boards", "timber"}},
	{Code: "29250", Description: "Cardboard cartons / paper packaging", Keywords: []string{"cardboard", "carton", "boxes", "packaging"}},
	{Code: "173680", Description: "Ceramic tile", Keywords: []string{"tile", "tiles", "ceramic", "porcelain"}},
	{Code: "86700", Description: "Glassware / glass products", Keywords: []string{"glass", "glassware", "mirror", "window glass"}},
	{Code: "157200", Description: "Rubber products", Keywords: []string{"rubber", "hose", "gasket", "rubber mat"}},
	{Code: "43700", Description: "Chemicals / adhesives in drums", Keywords: []string{"chemical", "solvent", "drum", "adhesive", "resin"}},
	{Code: "25120", Description: "Major appliances", Keywords: []string{"refrigerator", "washer", "dryer", "dishwasher", "appliance", "oven"}},
	{Code: "46400", Description: "Paint and coatings", Keywords: []string{"paint", "coating", "lacquer", "varnish"}},
	{Code: "60600", Description: "Pharmaceuticals / medical supplies", Keywords: []string{"pharmaceutical", "medicine", "medical", "drug", "vial"}},
	{Code: "180000", Description: "Sporting goods / exercise equipment", Keywords: []string{"sporting", "dumbbell", "weights", "treadmill", "exercise"}},
	{Code: "94500", Description: "Cosmetics / toiletries", Keywords: []string{"cosmetic", "shampoo", "soap", "toiletries", "lotion"}},
	{Code: "120600", Description: "Sheet metal / metal coil", Keywords: []string{"sheet metal", "steel coil", "aluminum sheet", "metal sheet"}},
	{Code: "133200", Description: "Bricks / masonry", Keywords: []string{"brick", "bricks", "masonry", "concrete block", "pavers"}},
}

// KeywordMatcher scores the curated catalog by keyword occurrences. Offline,
// deterministic, and dependency-free — it powers the demo and serves as the
// fallback for the LLM matcher.
type KeywordMatcher struct{}

// Match returns the best-matching catalog item for a free-text description,
// scored by whole-word/phrase keyword matches (so "tire" won't match inside
// "entire"). ok is false when nothing matches.
func (KeywordMatcher) Match(_ context.Context, description string) (NMFCItem, bool, error) {
	d := normalizeTokens(description)
	if strings.TrimSpace(d) == "" {
		return NMFCItem{}, false, nil
	}

	best := -1
	bestScore := 0
	for i, item := range nmfcCatalog {
		score := 0
		for _, kw := range item.Keywords {
			k := normalizeTokens(kw)
			if k != " " && strings.Contains(d, k) {
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

// normalizeTokens lowercases s, turns every non-alphanumeric run into a single
// space, and wraps the result in spaces. Space-wrapping lets strings.Contains
// enforce whole-word/phrase boundaries (" tire " is not inside " entire ").
func normalizeTokens(s string) string {
	var b strings.Builder
	b.WriteByte(' ')
	prevSpace := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	if !prevSpace {
		b.WriteByte(' ')
	}
	return b.String()
}

// matchByCode returns the catalog item whose NMFC code appears as a token in the
// text (e.g. "86150" or "NMFC 86150"). Exact-code entry is deterministic and
// takes precedence over keyword/LLM matching.
func matchByCode(text string) (NMFCItem, bool) {
	for _, tok := range strings.Fields(text) {
		tok = strings.Trim(tok, ".,:;#()[]{}")
		if item, ok := catalogByCode(tok); ok {
			return item, true
		}
	}
	return NMFCItem{}, false
}
