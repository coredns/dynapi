package dynapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// BenchmarkServeHTTP includes request construction and response encoding.
// The in-memory backend keeps DNS transfer and disk costs out of these timings.
func BenchmarkServeHTTP(b *testing.B) {
	cases := []struct {
		name, method, body, token string
		status                    int
	}{
		{"GET", http.MethodGet, "", "secret", http.StatusOK},
		{"PUT", http.MethodPut, `{"ttl":60,"addresses":["192.0.2.1"]}`, "secret", http.StatusOK},
		{"DELETE", http.MethodDelete, "", "secret", http.StatusNoContent},
		{"unauthorized", http.MethodGet, "", "wrong", http.StatusUnauthorized},
		{"invalid_address", http.MethodPut, `{"ttl":60,"addresses":["invalid"]}`, "secret", 422},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			backend := &testBackend{result: recordSet{TTL: 60, Addresses: []string{"192.0.2.1"}}}
			handler := newHandler("example.org.", "secret", backend, defaultMaxRequests)

			b.ReportAllocs()

			for b.Loop() {
				request := httptest.NewRequestWithContext(
					b.Context(),
					test.method,
					"/v1/zones/example.org/records/host.example.org/A",
					strings.NewReader(test.body),
				)
				request.Header.Set("Authorization", "Bearer "+test.token)
				request.Header.Set("Content-Type", "application/json")

				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)

				if response.Code != test.status {
					b.Fatalf("status=%d; want %d", response.Code, test.status)
				}
			}
		})
	}
}
