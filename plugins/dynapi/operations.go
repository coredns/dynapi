package dynapi

import (
	"fmt"
	"net/http"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

type operation struct {
	method, id, summary, description string
	errors                           []int
	status                           int
}

func (operation *operation) reflect(reflector *openapi31.Reflector) error {
	context, err := reflector.NewOperationContext(operation.method, recordPath)
	if err != nil {
		return fmt.Errorf("create OpenAPI operation: %w", err)
	}

	context.SetID(operation.id)
	context.SetSummary(operation.summary)
	context.SetDescription(operation.description)
	context.AddSecurity("bearerAuth")
	context.AddReqStructure(recordInput{})

	if operation.method == http.MethodPut {
		context.AddReqStructure(
			recordPayload{},
			openapi.WithCustomize(func(content openapi.ContentOrReference) {
				if body, ok := content.(*openapi31.RequestBodyOrReference); ok {
					body.RequestBodyEns().WithRequired(true)
				}
			}),
		)
	}

	if operation.status == http.StatusNoContent {
		context.AddRespStructure(nil, openapi.WithHTTPStatus(operation.status))
	} else {
		context.AddRespStructure(recordSet{}, openapi.WithHTTPStatus(operation.status))
	}

	for _, status := range operation.errors {
		addErrorResponse(context, reflector, status)
	}

	if err := reflector.AddOperation(context); err != nil {
		return fmt.Errorf("reflect OpenAPI operation: %w", err)
	}

	return nil
}

func recordOperations() []operation {
	common := []int{
		http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity, http.StatusBadGateway, http.StatusServiceUnavailable,
	}

	return []operation{
		{
			method:  http.MethodGet,
			id:      "getRecordSet",
			summary: "Read an exact stored record set",
			status:  http.StatusOK,
			errors:  common,
			description: fmt.Sprintf(
				"Read a signed AXFR snapshot without wildcard expansion or CNAME traversal. "+
					"Return the lowest stored TTL. Limit: %d records, including the repeated SOA, and %d bytes.",
				maxTransferRecords,
				maxTransferBytes,
			),
		},
		{
			method:  http.MethodPut,
			id:      "replaceRecordSet",
			summary: "Atomically replace a complete record set",
			status:  http.StatusOK,
			errors: append(
				append([]int{}, common...),
				http.StatusBadRequest,
				http.StatusForbidden,
				http.StatusConflict,
				http.StatusRequestEntityTooLarge,
				http.StatusUnsupportedMediaType,
			),
			description: fmt.Sprintf("Replace only the requested type. Body limit: %d bytes. "+
				"Addresses must match the type and are canonicalized, sorted, and deduplicated. "+
				"IPv4-mapped and scoped IPv6 addresses are rejected. TTL controls caching and does not expire records.",
				maxRequestBytes),
		},
		{
			method:  http.MethodDelete,
			id:      "deleteRecordSet",
			summary: "Delete one record type's complete set",
			status:  http.StatusNoContent,
			errors: append(
				append([]int{}, common...),
				http.StatusForbidden,
				http.StatusConflict,
			),
			description: "Leave other record types intact. Deleting an absent set succeeds.",
		},
	}
}
