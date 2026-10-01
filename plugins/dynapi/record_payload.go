package dynapi

import (
	"encoding/json/v2"
	"errors"
	"mime"
	"net/http"

	"github.com/swaggest/jsonschema-go"
)

const maxRequestBytes = 64 << 10

type recordPayload struct {
	TTL       *uint32  `json:"ttl"`
	Addresses []string `json:"addresses"`
}

// PrepareJSONSchema uses the same bounds checked by normalize.
func (*recordPayload) PrepareJSONSchema(schema *jsonschema.Schema) error {
	ttl := schema.Properties["ttl"].TypeObject
	addresses := schema.Properties["addresses"].TypeObject

	if ttl == nil || addresses == nil {
		return nil
	}

	schema.WithRequired("ttl", "addresses")
	schema.AdditionalPropertiesEns().WithTypeBoolean(false)
	ttl.WithType(jsonschema.Type{SimpleTypes: new(jsonschema.Integer)}).
		WithMaximum(maxTTL)
	addresses.WithType(jsonschema.Type{SimpleTypes: new(jsonschema.Array)}).
		WithMinItems(1).
		WithMaxItems(maxAddresses)

	return nil
}

func decodeRecordSet(w http.ResponseWriter, r *http.Request, rrtype uint16) (recordSet, error) {
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return recordSet{}, &apiError{
				status:  http.StatusUnsupportedMediaType,
				code:    codeUnsupportedContentType,
				message: "Content-Type must be application/json",
			}
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)

	var payload recordPayload

	if err := json.UnmarshalRead(r.Body, &payload, json.RejectUnknownMembers(true)); err != nil {
		return recordSet{}, invalidBody(err)
	}

	if payload.TTL == nil {
		return recordSet{}, invalidRecordSet()
	}

	result := recordSet{TTL: *payload.TTL, Addresses: payload.Addresses}
	if err := result.normalize(rrtype); err != nil {
		return recordSet{}, err
	}

	return result, nil
}

func invalidAddress() error {
	return &apiError{
		status: http.StatusUnprocessableEntity,
		code:   codeInvalidAddress, message: "addresses must match the requested IP family",
	}
}

func invalidBody(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return &apiError{
			status: http.StatusRequestEntityTooLarge,
			code:   codeBodyTooLarge, message: "request body exceeds 64 KiB",
		}
	}

	return &apiError{
		status:  http.StatusBadRequest,
		code:    codeInvalidJSON,
		message: "body must be one JSON object containing ttl and addresses",
	}
}

func invalidRecordSet() error {
	return &apiError{
		status:  http.StatusUnprocessableEntity,
		code:    codeInvalidRecordSet,
		message: "ttl must be 0..2147483647 and addresses must contain 1..256 values",
	}
}
