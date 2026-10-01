package main

import "fmt"

// apiError preserves the stable code for programmatic decisions.
// Keep unknown codes as errors so newer server responses remain safe to handle.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"error"`

	Status int `json:"-"`
}

func (apiError *apiError) Error() string {
	return fmt.Sprintf("HTTP %d (%s): %s", apiError.Status, apiError.Code, apiError.Message)
}
