package dynapi

import (
	"fmt"

	"github.com/swaggest/openapi-go/openapi31"
)

// GenerateOpenAPI exports the same operations and models used by the HTTP listener.
func GenerateOpenAPI() ([]byte, error) {
	reflector := openapi31.NewReflector()
	reflector.Spec.Info.WithTitle("CoreDNS dynapi").WithVersion("1").WithDescription(
		"Manage address records through signed DNS requests. Reads cover the configured zone. " +
			"Writes follow dynupdate permissions. A lost response may follow a committed write. " +
			"There are no automatic retries or conditional writes.")
	reflector.Spec.SetHTTPBearerTokenSecurity(
		"bearerAuth",
		"",
		"Private token configured by token or token_env.",
	)

	reflector.Spec.Servers = []openapi31.Server{{URL: "http://127.0.0.1:8080"}}

	for _, operation := range recordOperations() {
		if err := operation.reflect(reflector); err != nil {
			return nil, err
		}
	}

	document, err := reflector.Spec.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("serialize OpenAPI: %w", err)
	}

	return document, nil
}
