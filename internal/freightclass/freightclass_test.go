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
