package dynapi

import (
	"encoding/json/v2"
	"errors"
	"net/http"
)

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Error: message})
}

func writeFailure(w http.ResponseWriter, err error) {
	if failure, ok := errors.AsType[*apiError](err); ok {
		writeError(w, failure.status, failure.code, failure.message)

		return
	}

	// A lost response can follow a committed write. Do not suggest retrying it.
	writeError(
		w,
		http.StatusBadGateway,
		codeBackendFailure,
		"backend request failed; a write may already have committed",
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.MarshalWrite(w, value)
	if err != nil {
		log.Debugf("HTTP response could not be written: %v", err)
	}
}
