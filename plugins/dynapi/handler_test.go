package dynapi

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

//nolint:funlen,maintidx // Keep the cases local. Fixture data inflates size metrics.
func TestServeHTTP(t *testing.T) {
	t.Parallel()

	for _, maxRequests := range []int{1, 3, defaultMaxRequests} {
		t.Run(fmt.Sprintf("concurrent admission and recovery/%d", maxRequests), func(t *testing.T) {
			t.Parallel()
			checkConcurrentAdmission(t, maxRequests)
		})
	}

	cases := []struct {
		failure                                            error
		name, path, body, token, contentType, method, code string
		result                                             recordSet
		status, writes, active                             int
	}{
		{
			method: http.MethodPut, name: "unauthorized", code: codeUnauthorized,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token:       "wrong",
			contentType: "application/json",
			status:      401,
		},
		{
			method: http.MethodPut, name: "outside zone", code: codeInvalidName,
			path:        "/v1/zones/example.org/records/host.other.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "suffix is not a label boundary", code: codeInvalidName,
			path:  "/v1/zones/example.org/records/notexample.org/A",
			body:  `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token: "secret", contentType: "application/json", status: 422,
		},

		{
			method: http.MethodPut, name: "wildcard", code: codeInvalidName,
			path:        "/v1/zones/example.org/records/*.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "other zone", code: codeZoneNotFound,
			path:        "/v1/zones/other.org/records/host.other.org/A",
			body:        `{}`,
			token:       "secret",
			contentType: "application/json",
			status:      404,
		},
		{
			method: http.MethodPut, name: "wrong family", code: codeInvalidAddress,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["2001:db8::1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "mapped IPv6", code: codeInvalidAddress,
			path:        "/v1/zones/example.org/records/host.example.org/AAAA",
			body:        `{"ttl":60,"addresses":["::ffff:192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "scoped IPv6", code: codeInvalidAddress,
			path:        "/v1/zones/example.org/records/host.example.org/AAAA",
			body:        `{"ttl":60,"addresses":["fe80::1%lo0"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "missing ttl", code: codeInvalidRecordSet,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "large ttl", code: codeInvalidRecordSet,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":2147483648,"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "unknown field", code: codeInvalidJSON,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"],"extra":true}`,
			token:       "secret",
			contentType: "application/json",
			status:      400,
		},
		{
			method: http.MethodPut, name: "duplicate JSON key", code: codeInvalidJSON,
			path:  "/v1/zones/example.org/records/host.example.org/A",
			body:  `{"ttl":60,"ttl":120,"addresses":["192.0.2.1"]}`,
			token: "secret", contentType: "application/json", status: http.StatusBadRequest,
		},
		{
			method: http.MethodPut, name: "case-sensitive JSON keys", code: codeInvalidJSON,
			path:  "/v1/zones/example.org/records/host.example.org/A",
			body:  `{"TTL":60,"addresses":["192.0.2.1"]}`,
			token: "secret", contentType: "application/json", status: http.StatusBadRequest,
		},
		{
			method: http.MethodPut, name: "invalid UTF-8", code: codeInvalidJSON,
			path:  "/v1/zones/example.org/records/host.example.org/A",
			body:  "{\"ttl\":60,\"addresses\":[\"" + string([]byte{0xff}) + "\"]}",
			token: "secret", contentType: "application/json", status: http.StatusBadRequest,
		},
		{
			method: http.MethodPut, name: "trailing whitespace",
			path:  "/v1/zones/example.org/records/host.example.org/A",
			body:  "{\"ttl\":60,\"addresses\":[\"192.0.2.1\"]} \n\t",
			token: "secret", contentType: "application/json", status: http.StatusOK, writes: 1,
			result: recordSet{TTL: 60, Addresses: []string{"192.0.2.1"}},
		},

		{
			method: http.MethodPut, name: "trailing JSON", code: codeInvalidJSON,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}{}`,
			token:       "secret",
			contentType: "application/json",
			status:      400,
		},
		{
			method: http.MethodPut, name: "empty set", code: codeInvalidRecordSet,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":[]}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "unsupported type", code: codeUnsupportedRecordType,
			path:        "/v1/zones/example.org/records/host.example.org/TXT",
			body:        `{}`,
			token:       "secret",
			contentType: "application/json",
			status:      422,
		},
		{
			method: http.MethodPut, name: "wrong media type", code: codeUnsupportedContentType,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{}`,
			token:       "secret",
			contentType: "text/plain",
			status:      415,
		},
		{
			method:      http.MethodPut,
			name:        "JSON with charset",
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json; charset=utf-8",
			status:      http.StatusOK,
			writes:      1,
			result:      recordSet{TTL: 60, Addresses: []string{"192.0.2.1"}},
		},
		{
			method:      http.MethodPut,
			name:        "malformed content type",
			code:        codeUnsupportedContentType,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["192.0.2.1"]}`,
			token:       "secret",
			contentType: "application/json; charset",
			status:      http.StatusUnsupportedMediaType,
		},

		{
			method: http.MethodPut, name: "large body", code: codeBodyTooLarge,
			path:        "/v1/zones/example.org/records/host.example.org/A",
			body:        `{"ttl":60,"addresses":["` + strings.Repeat("x", 64<<10) + `"]}`,
			token:       "secret",
			contentType: "application/json",
			status:      413,
		},
		{
			name: "normalize addresses", method: http.MethodPut,
			path:  "/v1/zones/EXAMPLE.ORG/records/Host.Example.Org/AAAA",
			body:  `{"ttl":0,"addresses":["2001:db8:0::2","2001:db8::1","2001:db8::2"]}`,
			token: "secret", contentType: "application/json", status: http.StatusOK, writes: 1,
			result: recordSet{TTL: 0, Addresses: []string{"2001:db8::1", "2001:db8::2"}},
		},
		{
			name:   "backend conflict",
			code:   codeRecordConflict,
			method: http.MethodDelete,
			path:   "/v1/zones/example.org/records/host.example.org/A",
			token:  "secret",
			status: http.StatusConflict,
			writes: 1,
			failure: &apiError{
				status:  http.StatusConflict,
				code:    codeRecordConflict,
				message: "CNAME exists",
			},
		},
		{
			name: "admission limit", code: codeTooManyRequests, method: http.MethodDelete,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusServiceUnavailable, active: defaultMaxRequests,
		},
		{
			name:   "missing set",
			method: http.MethodGet,
			code:   codeRecordSetNotFound,
			path:   "/v1/zones/example.org/records/host.example.org/A",
			token:  "secret",
			status: http.StatusNotFound,
			failure: &apiError{
				status:  http.StatusNotFound,
				code:    codeRecordSetNotFound,
				message: "record set not found",
			},
		},
		{
			name: "transport failure", method: http.MethodGet, code: codeBackendFailure,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusBadGateway, failure: errUnsignedResponse,
		},
		{
			name: "read", method: http.MethodGet,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusOK, result: recordSet{TTL: 60, Addresses: []string{"192.0.2.1"}},
		},
		{
			name: "delete", method: http.MethodDelete,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusNoContent, writes: 1,
		},
		{
			name: "HEAD", code: codeMethodNotAllowed, method: http.MethodHead,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusMethodNotAllowed,
		},
		{
			name: "unknown path", code: codeResourceNotFound, method: http.MethodGet,
			path: "/unknown", token: "secret", status: http.StatusNotFound,
		},
		{
			name: "unsupported method", code: codeMethodNotAllowed, method: http.MethodPost,
			path: "/v1/zones/example.org/records/host.example.org/A", token: "secret",
			status: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			backend := &testBackend{err: test.failure, result: test.result}
			handler := newHandler("example.org.", "secret", backend, defaultMaxRequests)

			handler.slots = make(chan struct{}, defaultMaxRequests-test.active)

			r := httptest.NewRequestWithContext(
				t.Context(),
				test.method,
				test.path,
				strings.NewReader(test.body),
			)
			r.Header.Set("Authorization", "Bearer "+test.token)
			r.Header.Set("Content-Type", test.contentType)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != test.status || backend.writes != test.writes {
				t.Fatalf("status=%d writes=%d body=%s; want status=%d writes=%d",
					w.Code, backend.writes, w.Body, test.status, test.writes)
			}

			checkErrorResponse(t, w.Body.Bytes(), test.code)

			if test.status != http.StatusOK {
				return
			}

			var got recordSet

			err := json.Unmarshal(w.Body.Bytes(), &got)
			if err != nil || !reflect.DeepEqual(got, test.result) {
				t.Fatalf("result=%+v error=%v; want %+v", got, err, test.result)
			}
		})
	}
}

func checkErrorResponse(t *testing.T, body []byte, code string) {
	t.Helper()

	if code == "" {
		return
	}

	var failure errorResponse

	if err := json.Unmarshal(body, &failure); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if failure.Code != code || failure.Error == "" {
		t.Fatalf("error=%+v; want code=%s and a message", failure, code)
	}
}

func checkConcurrentAdmission(t *testing.T, maxRequests int) {
	t.Helper()

	const path = "/v1/zones/example.org/records/host.example.org/A"

	backend := &blockingBackend{
		started: make(chan struct{}, maxRequests), release: make(chan struct{}),
	}
	handler := newHandler("example.org.", "secret", backend, maxRequests)

	var requests sync.WaitGroup

	// Cleanup releases blocked readers even if an assertion fails.
	defer requests.Wait()
	defer func() {
		select {
		case <-backend.release:
		default:
			close(backend.release)
		}
	}()

	for range maxRequests {
		requests.Go(func() { checkAdmittedRequest(t, handler) })
	}

	for range maxRequests {
		select {
		case <-backend.started:
		case <-time.After(requestTimeout):
			t.Fatal("requests did not reach the backend")
		}
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("saturated handler returned %d", response.Code)
	}

	checkErrorResponse(t, response.Body.Bytes(), codeTooManyRequests)

	close(backend.release)
	requests.Wait()

	checkAdmittedRequest(t, handler)
}

func checkAdmittedRequest(t *testing.T, handler *handler) {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/v1/zones/example.org/records/host.example.org/A", http.NoBody)
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("admitted request returned %d: %s", response.Code, response.Body)
	}
}
