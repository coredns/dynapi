package dynapi

import (
	"net/http"
	"strings"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

const (
	codeUnauthorized           = "unauthorized"
	codeInvalidJSON            = "invalid_json"
	codeUpdateDenied           = "update_denied"
	codeZoneNotFound           = "zone_not_found"
	codeRecordSetNotFound      = "record_set_not_found"
	codeResourceNotFound       = "resource_not_found"
	codeMethodNotAllowed       = "method_not_allowed"
	codeRecordConflict         = "record_conflict"
	codeBodyTooLarge           = "body_too_large"
	codeUnsupportedContentType = "unsupported_content_type"
	codeInvalidName            = "invalid_name"
	codeUnsupportedRecordType  = "unsupported_record_type"
	codeInvalidAddress         = "invalid_address"
	codeInvalidRecordSet       = "invalid_record_set"
	codeBackendFailure         = "backend_failure"
	codeBackendRejected        = "backend_rejected"
	codeReadLimitExceeded      = "read_limit_exceeded"
	codeTooManyRequests        = "too_many_requests"
)

type errorResponse struct {
	Code  string `json:"code"  required:"true"`
	Error string `json:"error" required:"true"`
}

func responseCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest:   {codeInvalidJSON},
		http.StatusUnauthorized: {codeUnauthorized},
		http.StatusForbidden:    {codeUpdateDenied},
		http.StatusNotFound: {
			codeZoneNotFound,
			codeRecordSetNotFound,
			codeResourceNotFound,
		},
		http.StatusMethodNotAllowed:      {codeMethodNotAllowed},
		http.StatusConflict:              {codeRecordConflict},
		http.StatusRequestEntityTooLarge: {codeBodyTooLarge},
		http.StatusUnsupportedMediaType:  {codeUnsupportedContentType},
		http.StatusUnprocessableEntity: {
			codeInvalidName, codeUnsupportedRecordType, codeInvalidAddress, codeInvalidRecordSet,
		},
		http.StatusBadGateway: {
			codeBackendFailure,
			codeBackendRejected,
			codeReadLimitExceeded,
		},
		http.StatusServiceUnavailable: {codeTooManyRequests},
	}
}

func addErrorResponse(
	context openapi.OperationContext,
	reflector *openapi31.Reflector,
	status int,
) {
	context.AddRespStructure(errorResponse{}, openapi.WithHTTPStatus(status),
		openapi.WithCustomize(func(content openapi.ContentOrReference) {
			response, ok := content.(*openapi31.ResponseOrReference)
			if !ok {
				return
			}

			media := response.ResponseEns().Content["application/json"]
			name := strings.ReplaceAll(http.StatusText(status), " ", "") + "Response"
			reflector.Spec.ComponentsEns().WithSchemasItem(name, map[string]any{
				"allOf": []any{
					media.Schema,
					map[string]any{"type": "object", "properties": map[string]any{
						"code": map[string]any{"type": "string", "enum": responseCodes()[status]},
					}},
				},
			})

			media.Schema = map[string]any{"$ref": "#/components/schemas/" + name}
			response.ResponseEns().Content["application/json"] = media
		}),
	)
}
