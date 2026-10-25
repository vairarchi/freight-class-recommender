package freightclass

import "fmt"

// savingsThreshold is the largest fraction of volume we will suggest removing
// for a packing tip to be realistic (~15%).
const savingsThreshold = 0.15

// PackingSavings describes a legitimately reachable cheaper class via tighter
// packing (denser = lower class = lower cost).
type PackingSavings struct {
	TargetClass     Class
	RequiredDensity float64
	CubeReduction   float64 // cubic feet to remove
	HeightReduction float64 // inches off height (0 when dims unknown)
	Message         string
}

// SavingsOpportunity returns a packing tip when a modest volume reduction would
// raise density past the next breakpoint into a cheaper class.
func SavingsOpportunity(in Input, density, cube float64, current Class) (PackingSavings, bool) {
	if density <= 0 || cube <= 0 || in.WeightLbs <= 0 {
		return PackingSavings{}, false
	}

	hi := nextHigherBreakpoint(density)
	if hi <= 0 {
		return PackingSavings{}, false // already in the densest/cheapest class
	}
	target := ClassForDensity(hi)
	if target >= current {
		return PackingSavings{}, false
	}

	requiredCube := in.WeightLbs / hi
	cubeReduction := cube - requiredCube
	if cubeReduction <= 0 {
		return PackingSavings{}, false
	}
	frac := cubeReduction / cube
	if frac > savingsThreshold {
		return PackingSavings{}, false // too much repacking to be realistic
	}

	ps := PackingSavings{
		TargetClass:     target,
		RequiredDensity: hi,
		CubeReduction:   cubeReduction,
	}
	// Express as a height reduction only when dims (not a supplied cube) drive volume.
	if in.Cube == nil && in.Length > 0 && in.Width > 0 && in.Height > 0 {
		newHeight := requiredCube * 1728 / (in.Length * in.Width)
		ps.HeightReduction = in.Height - newHeight
	}

	ps.Message = fmt.Sprintf(
		"Tighten packing by ~%.1f cu ft (≈%.0f%%) to raise density to %s lb/ft³ and drop from class %s to %s — a lower, cheaper class.",
		ps.CubeReduction, frac*100, formatDensity(ps.RequiredDensity), formatClass(current), formatClass(target),
	)
	if ps.HeightReduction > 0 {
		ps.Message += fmt.Sprintf(" e.g. reduce height by ~%.1f in.", ps.HeightReduction)
	}
	return ps, true
}

// nextHigherBreakpoint returns the smallest density cutoff strictly above
// density (crossing it yields a lower, cheaper class), or -1 if none.
func nextHigherBreakpoint(density float64) float64 {
	best := -1.0
	for _, b := range densityScale {
		if b.minDensity > density && (best < 0 || b.minDensity < best) {
			best = b.minDensity
		}
	}
	return best
}
