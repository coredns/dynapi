package dynapi

import (
	"encoding/json/v2"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go/openapi31"
)

func TestGenerateOpenAPI(t *testing.T) {
	t.Parallel()

	document, err := GenerateOpenAPI()
	if err != nil {
		t.Fatal(err)
	}

	var spec openapi31.Spec

	if err := spec.UnmarshalYAML(document); err != nil {
		t.Fatal(err)
	}

	for status, codes := range responseCodes() {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			name := strings.ReplaceAll(http.StatusText(status), " ", "") + "Response"

			schema, err := json.Marshal(spec.Components.Schemas[name])
			if err != nil {
				t.Fatal(err)
			}

			checkResponseSchema(t, name, schema, codes)
		})
	}
}

func checkResponseSchema(t *testing.T, name string, schema []byte, codes []string) {
	t.Helper()

	var response jsonschema.Schema

	if err := json.Unmarshal(schema, &response); err != nil {
		t.Fatal(err)
	}

	if len(response.AllOf) != 2 || response.AllOf[1].TypeObject == nil {
		t.Fatalf("%s has no status-specific restriction: %s", name, schema)
	}

	constraint := response.AllOf[1].TypeObject.Properties["code"].TypeObject
	if constraint == nil {
		t.Fatalf("%s has no code property: %s", name, schema)
	}

	values := make([]any, len(codes))
	for index, code := range codes {
		values[index] = code
	}

	if !reflect.DeepEqual(constraint.Enum, values) {
		t.Fatalf("%s does not restrict error codes to %v: %s", name, codes, schema)
	}
}
