package freightclass

// Class is an NMFC freight class value (50–500).
type Class float64

// RiskLevel describes how close a density is to a class breakpoint.
type RiskLevel string

const (
	RiskNone   RiskLevel = "none"
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Input is the request to the recommendation engine. Provide either Cube
// (trusted dimensioner value) or L/W/H in inches. Weight is in pounds.
type Input struct {
	Length        float64  `json:"length"`
	Width         float64  `json:"width"`
	Height        float64  `json:"height"`
	Cube          *float64 `json:"cube,omitempty"` // nil = compute from dimensions
	WeightLbs     float64  `json:"weightLbs"`
	SelectedClass *float64 `json:"selectedClass,omitempty"` // nil = user has not chosen yet
	Description   string   `json:"description,omitempty"`   // free-text goods description for NMFC match
}

// Recommendation is the UI contract returned by the recommend engine.
type Recommendation struct {
	RecommendedClass float64   `json:"recommendedClass"`
	Density          float64   `json:"density"`
	SelectedClass    *float64  `json:"selectedClass"`
	State            string    `json:"state"` // suggest | confirm | save | warn
	Message          string    `json:"message"`
	RiskLevel        RiskLevel `json:"riskLevel"`
	CostTip          string    `json:"costTip"`
	NMFCCode         string    `json:"nmfcCode,omitempty"`
	NMFCDescription  string    `json:"nmfcDescription,omitempty"`
	NMFCFixedClass   bool      `json:"nmfcFixedClass"`
}
