package freightclass

import (
	"fmt"
	"math"
	"strconv"
)

// boundaryThreshold is the fraction of a breakpoint within which a density is
// flagged as a boundary risk (~5%).
const boundaryThreshold = 0.05

// densityScale maps a minimum density (inclusive, lb/ft³) to its NMFC class,
// ordered from densest/cheapest to lightest/most expensive. The final entry
// (minDensity 0) is the catch-all for anything under 1 lb/ft³.
var densityScale = []struct {
	minDensity float64
	class      Class
}{
	{50, 50},
	{35, 55},
	{30, 60},
	{22.5, 65},
	{15, 70},
	{13.5, 77.5},
	{12, 85},
	{10.5, 92.5},
	{9, 100},
	{8, 110},
	{7, 125},
	{6, 150},
	{5, 175},
	{4, 200},
	{3, 250},
	{2, 300},
	{1, 400},
	{0, 500},
}

// CubeFromDims returns volume in cubic feet from inch dimensions.
// Used only when a dimensioner cube is not supplied.
func CubeFromDims(l, w, h float64) float64 {
	return (l * w * h) / 1728.0
}

// Density returns weight per cubic foot, or 0 when the cube is non-positive.
func Density(weightLbs, cubeFt float64) float64 {
	if cubeFt <= 0 {
		return 0
	}
	return weightLbs / cubeFt
}

// ClassForDensity maps a density to its NMFC class via the density scale.
func ClassForDensity(density float64) Class {
	for _, b := range densityScale {
		if density >= b.minDensity {
			return b.class
		}
	}
	return 500
}

// BoundaryRisk reports whether a density sits near a class breakpoint and, when
// it does, an explanatory message naming the two adjacent classes.
func BoundaryRisk(density float64) (RiskLevel, string) {
	if density <= 0 {
		return RiskNone, ""
	}

	nearest, margin := nearestBreakpoint(density)
	if nearest <= 0 || margin > boundaryThreshold {
		return RiskNone, ""
	}

	lower := ClassForDensity(nearest - 1e-9) // class just below the cutoff
	upper := ClassForDensity(nearest)        // class at/above the cutoff
	msg := fmt.Sprintf(
		"Density %s is near the %s cutoff. A small measurement difference could reclassify this between class %s and class %s. Double-check dimensions/weight.",
		formatDensity(density), formatDensity(nearest), formatClass(upper), formatClass(lower),
	)

	switch {
	case margin <= 0.015:
		return RiskHigh, msg
	case margin <= 0.03:
		return RiskMedium, msg
	default:
		return RiskLow, msg
	}
}

// nearestBreakpoint returns the closest density cutoff and the relative margin
// to it. The 0 catch-all is not a real cutoff and is ignored.
func nearestBreakpoint(density float64) (breakpoint, margin float64) {
	margin = math.MaxFloat64
	for _, b := range densityScale {
		if b.minDensity <= 0 {
			continue
		}
		m := math.Abs(b.minDensity-density) / b.minDensity
		if m < margin {
			margin = m
			breakpoint = b.minDensity
		}
	}
	return breakpoint, margin
}

// Recommend runs the full pipeline: resolve cube, compute density, look up the
// class, apply any NMFC fixed-class override, check boundary risk, and compare
// against the user's selection.
func Recommend(in Input) Recommendation {
	cube := CubeFromDims(in.Length, in.Width, in.Height)
	if in.Cube != nil && *in.Cube > 0 {
		cube = *in.Cube
	}

	density := Density(in.WeightLbs, cube)
	recommended := ClassForDensity(density)

	rec := Recommendation{
		Density:       density,
		SelectedClass: in.SelectedClass,
	}

	// NMFC free-text match: a fixed-class item overrides the density class.
	fixedClass := false
	if item, ok := MatchNMFC(in.Description); ok {
		rec.NMFCCode = item.Code
		rec.NMFCDescription = item.Description
		if item.FixedClass != nil {
			recommended = *item.FixedClass
			rec.NMFCFixedClass = true
			fixedClass = true
		}
	}
	rec.RecommendedClass = float64(recommended)

	// Boundary risk only applies to density-based classification.
	var riskMsg string
	if !fixedClass {
		rec.RiskLevel, riskMsg = BoundaryRisk(density)
	} else {
		rec.RiskLevel = RiskNone
	}

	classStr := formatClass(recommended)
	var basis string
	if fixedClass {
		basis = fmt.Sprintf("NMFC %s (%s) is a fixed-class item; class %s applies regardless of density",
			rec.NMFCCode, rec.NMFCDescription, classStr)
	} else {
		basis = fmt.Sprintf("based on density %s lb/ft³", formatDensity(density))
	}

	switch {
	case in.SelectedClass == nil:
		rec.State = "suggest"
		rec.Message = fmt.Sprintf("Suggested class %s — %s.", classStr, basis)
	case *in.SelectedClass == float64(recommended):
		rec.State = "confirm"
		rec.Message = fmt.Sprintf("Class %s matches — %s.", classStr, basis)
	case *in.SelectedClass > float64(recommended):
		rec.State = "save"
		rec.Message = fmt.Sprintf("Class %s may be higher than needed (%s). You can likely use class %s.",
			formatClass(Class(*in.SelectedClass)), basis, classStr)
		rec.CostTip = fmt.Sprintf("You can likely use class %s — lower cost, still valid.", classStr)
	default:
		rec.State = "warn"
		rec.Message = fmt.Sprintf("Class %s may be reclassified to %s (%s); too-low declaration risks fees.",
			formatClass(Class(*in.SelectedClass)), classStr, basis)
	}

	if riskMsg != "" {
		rec.Message = rec.Message + " " + riskMsg
	}

	return rec
}

func formatDensity(d float64) string {
	return strconv.FormatFloat(d, 'f', 1, 64)
}

func formatClass(c Class) string {
	return strconv.FormatFloat(float64(c), 'g', -1, 64)
}
