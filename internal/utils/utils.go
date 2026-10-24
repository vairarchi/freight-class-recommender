package utils

import (
	"encoding/json"
	"net/http"

	"gitlab.com/pitneybowes/innovation/pbgolib/resterrors"
)

// GetPayload decodes the JSON request body into req, returning a validation
// rest error on a nil body or malformed JSON.
func GetPayload(r *http.Request, req interface{}) error {
	op := "utils.GetPayload"

	if r.Body == nil {
		return &resterrors.Error{Op: op, Code: resterrors.ERR_VALIDATION, Message: "Invalid request body. Is nil", StatusCode: http.StatusBadRequest}
	}

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return &resterrors.Error{Op: op, Code: resterrors.ERR_VALIDATION, Message: "Invalid request body", StatusCode: http.StatusBadRequest}
	}
	return nil
}
