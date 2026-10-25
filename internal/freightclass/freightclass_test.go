package freightclass

import (
	"math"
	"strings"
	"testing"
)

func ptr(f float64) *float64 { return &f }

func TestCubeFromDims(t *testing.T) {
	// 12 x 12 x 12 in = 1 cu ft
	if got := CubeFromDims(12, 12, 12); math.Abs(got-1) > 1e-9 {
		t.Fatalf("CubeFromDims(12,12,12) = %v, want 1", got)
	}
	// 48 x 40 x 36 in = 69120 / 1728 = 40 cu ft
	if got := CubeFromDims(48, 40, 36); math.Abs(got-40) > 1e-9 {
		t.Fatalf("CubeFromDims(48,40,36) = %v, want 40", got)
	}
}

func TestDensity(t *testing.T) {
	if got := Density(100, 10); got != 10 {
		t.Fatalf("Density(100,10) = %v, want 10", got)
	}
	if got := Density(100, 0); got != 0 {
		t.Fatalf("Density(100,0) = %v, want 0 (divide-by-zero guard)", got)
	}
}

func TestClassForDensity(t *testing.T) {
	cases := []struct {
		density float64
		want    Class
	}{
		{60, 50},     // >= 50
		{50, 50},     // boundary inclusive
		{40, 55},     // 35–50
		{14.9, 77.5}, // just below 15
		{15, 70},     // boundary inclusive
		{9, 100},     // boundary inclusive
		{6.5, 150},   // 6–7
		{0.5, 500},   // < 1
		{0, 500},     // zero
	}
	for _, c := range cases {
		if got := ClassForDensity(c.density); got != c.want {
			t.Errorf("ClassForDensity(%v) = %v, want %v", c.density, got, c.want)
		}
	}
}

func TestBoundaryRisk(t *testing.T) {
	// 14.9 is ~0.67% below the 15.0 cutoff -> flagged, high.
	level, msg := BoundaryRisk(14.9)
	if level != RiskHigh {
		t.Errorf("BoundaryRisk(14.9) level = %v, want %v", level, RiskHigh)
	}
	if !strings.Contains(msg, "15.0") {
		t.Errorf("BoundaryRisk(14.9) message should mention the 15.0 cutoff, got %q", msg)
	}

	// 11.25 is the midpoint between 10.5 and 12 -> no flag.
	if level, _ := BoundaryRisk(11.25); level != RiskNone {
		t.Errorf("BoundaryRisk(11.25) level = %v, want %v", level, RiskNone)
	}

	// Non-positive density -> no flag.
	if level, _ := BoundaryRisk(0); level != RiskNone {
		t.Errorf("BoundaryRisk(0) level = %v, want %v", level, RiskNone)
	}
}

func TestRecommend(t *testing.T) {
	// density = 112.5 lbs / 10 cu ft = 11.25 -> class 92.5
	base := Input{Cube: ptr(10), WeightLbs: 112.5}

	// State A: no selection -> suggest
	if got := Recommend(base); got.State != "suggest" || got.RecommendedClass != 92.5 {
		t.Errorf("no selection: state=%q class=%v, want suggest/92.5", got.State, got.RecommendedClass)
	}

	// State B: match -> confirm
	in := base
	in.SelectedClass = ptr(92.5)
	if got := Recommend(in); got.State != "confirm" {
		t.Errorf("match: state=%q, want confirm", got.State)
	}

	// State C: over-declared (selected higher than needed) -> save
	in = base
	in.SelectedClass = ptr(150)
	if got := Recommend(in); got.State != "save" || got.CostTip == "" {
		t.Errorf("over-declared: state=%q costTip=%q, want save with tip", got.State, got.CostTip)
	}

	// State D: under-declared (selected lower than density supports) -> warn
	in = base
	in.SelectedClass = ptr(70)
	if got := Recommend(in); got.State != "warn" {
		t.Errorf("under-declared: state=%q, want warn", got.State)
	}
}

func TestRecommendUsesDimsWhenNoCube(t *testing.T) {
	// 48 x 40 x 36 = 40 cu ft; 450 lbs -> density 11.25 -> class 92.5
	in := Input{Length: 48, Width: 40, Height: 36, WeightLbs: 450}
	got := Recommend(in)
	if math.Abs(got.Density-11.25) > 1e-9 || got.RecommendedClass != 92.5 {
		t.Errorf("dims path: density=%v class=%v, want 11.25/92.5", got.Density, got.RecommendedClass)
	}
}

func TestMatchNMFC(t *testing.T) {
	if item, ok := MatchNMFC("box of ping pong balls"); !ok || item.FixedClass == nil || *item.FixedClass != 500 {
		t.Errorf("ping pong should match fixed class 500, got %+v ok=%v", item, ok)
	}
	if item, ok := MatchNMFC("assorted machine parts"); !ok || item.FixedClass != nil {
		t.Errorf("machine parts should match a density-based item, got %+v ok=%v", item, ok)
	}
	if _, ok := MatchNMFC("qwerty zxcv"); ok {
		t.Error("gibberish should not match")
	}
	if _, ok := MatchNMFC(""); ok {
		t.Error("empty should not match")
	}
}

func TestKeywordWholeWord(t *testing.T) {
	// "tire" must not match inside "entire".
	if _, ok := MatchNMFC("the entire shipment is fragile"); ok {
		t.Error("'entire' should not trigger a 'tire' keyword match")
	}
	// Exact word still matches.
	if item, ok := MatchNMFC("a set of spare tires"); !ok || item.Code != "99080" {
		t.Errorf("'tires' should match automotive 99080, got ok=%v code=%q", ok, item.Code)
	}
	// Multi-word phrase still matches.
	if item, ok := MatchNMFC("bulk packing peanuts for void fill"); !ok || item.Code != "156300" {
		t.Errorf("'packing peanuts' should match 156300, got ok=%v code=%q", ok, item.Code)
	}
}

func TestRecommendFixedClassOverride(t *testing.T) {
	// A dense box (class 50 by density) of ping pong balls -> fixed class 500.
	in := Input{Cube: ptr(1), WeightLbs: 60, Description: "ping pong balls"}
	got := Recommend(in)
	if got.RecommendedClass != 500 || !got.NMFCFixedClass {
		t.Errorf("fixed-class override: class=%v fixed=%v, want 500/true", got.RecommendedClass, got.NMFCFixedClass)
	}
	if got.RiskLevel != RiskNone {
		t.Errorf("fixed-class should have no boundary risk, got %v", got.RiskLevel)
	}
}

func TestRecommendDensityItemKeepsDensityClass(t *testing.T) {
	// A matched but density-rated item keeps the density class.
	in := Input{Cube: ptr(10), WeightLbs: 112.5, Description: "machine parts"}
	got := Recommend(in)
	if got.RecommendedClass != 92.5 || got.NMFCFixedClass {
		t.Errorf("density item: class=%v fixed=%v, want 92.5/false", got.RecommendedClass, got.NMFCFixedClass)
	}
	if got.NMFCCode == "" {
		t.Error("should still report the matched NMFC code")
	}
}

func TestNextHigherBreakpoint(t *testing.T) {
	if got := nextHigherBreakpoint(11.25); got != 12 {
		t.Errorf("nextHigherBreakpoint(11.25) = %v, want 12", got)
	}
	if got := nextHigherBreakpoint(60); got != -1 {
		t.Errorf("nextHigherBreakpoint(60) = %v, want -1 (no higher cutoff)", got)
	}
}

func TestSavingsOpportunity(t *testing.T) {
	// density 11.25 (class 92.5) -> reachable 85 at density 12 with ~6% repack.
	ps, ok := SavingsOpportunity(Input{Cube: ptr(12), WeightLbs: 135}, 11.25, 12, 92.5)
	if !ok || ps.TargetClass != 85 {
		t.Fatalf("expected savings to class 85, got ok=%v target=%v", ok, ps.TargetClass)
	}
	if ps.CubeReduction <= 0 || ps.Message == "" {
		t.Errorf("expected positive cube reduction and message, got %+v", ps)
	}
	// Already cheapest class (50) -> no tip.
	if _, ok := SavingsOpportunity(Input{Cube: ptr(1), WeightLbs: 60}, 60, 1, 50); ok {
		t.Error("class 50 should have no cheaper target")
	}
	// Too much repacking required -> no tip.
	if _, ok := SavingsOpportunity(Input{Cube: ptr(1), WeightLbs: 1.2}, 1.2, 1, 400); ok {
		t.Error("~40% repack should be rejected as unrealistic")
	}
}

func TestRecommendPackingTip(t *testing.T) {
	// 48x40x36 @ 450 lbs -> density 11.25 class 92.5, reachable 85.
	got := Recommend(Input{Length: 48, Width: 40, Height: 36, WeightLbs: 450})
	if got.PackingTip == "" || got.TargetClass != 85 {
		t.Errorf("expected packing tip to class 85, got tip=%q target=%v", got.PackingTip, got.TargetClass)
	}
	if !strings.Contains(got.PackingTip, "height") {
		t.Errorf("dims path should mention height reduction, got %q", got.PackingTip)
	}
}

func TestRecommendByCodePrecedence(t *testing.T) {
	// A declared NMFC code is authoritative and beats the description match.
	in := Input{Cube: ptr(10), WeightLbs: 112.5, NMFCCode: "86150", Description: "books"}
	got := Recommend(in)
	if got.RecommendedClass != 500 || !got.NMFCFixedClass || got.NMFCSource != "code" {
		t.Errorf("declared code should win: class=%v fixed=%v source=%q, want 500/true/code",
			got.RecommendedClass, got.NMFCFixedClass, got.NMFCSource)
	}
}

func TestRecommendUnknownCodeFallsBackToDescription(t *testing.T) {
	// An unknown code falls through to description matching.
	in := Input{Cube: ptr(10), WeightLbs: 112.5, NMFCCode: "99999", Description: "machine parts"}
	got := Recommend(in)
	if got.NMFCCode != "61000" || got.NMFCSource != "description" {
		t.Errorf("unknown code should fall back to description: code=%q source=%q, want 61000/description",
			got.NMFCCode, got.NMFCSource)
	}
}

func TestRecommendFixedClassNoPackingTip(t *testing.T) {
	got := Recommend(Input{Length: 48, Width: 40, Height: 36, WeightLbs: 450, Description: "ping pong balls"})
	if got.PackingTip != "" {
		t.Errorf("fixed-class item should not get a packing tip, got %q", got.PackingTip)
	}
}
