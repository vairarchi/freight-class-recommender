// Package freightclass implements the deterministic NMFC density-to-class
// calculator, boundary risk detection, the recommend state machine, and the
// HTTP API handlers that expose them.
package freightclass

import (
	"net/http"

	"github.com/vairarchi/freight-class-recommender/internal/utils"
	"gitlab.com/pitneybowes/innovation/pbgolib/core/logging"
	"gitlab.com/pitneybowes/innovation/pbgolib/respond"
)

// Context carries the dependencies for the freight class APIs.
type Context struct{}

// New creates the freight class Context.
func New() *Context {
	return &Context{}
}

// RecommendAPI computes a freight class recommendation from the supplied
// dimensions/cube, weight, and optional selected class. It is registered as an
// unauthenticated route for the demo, so it takes no token claims.
func (ctx *Context) RecommendAPI(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := utils.GetPayload(r, &in); err != nil {
		logging.WithError(r.Context(), "Error in GetPayload:", err)
		respond.WithErrorV2(w, r, err)
		return
	}

	rec := Recommend(in)
	respond.WithV2(w, r, http.StatusOK, rec)
}
