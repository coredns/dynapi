package dynapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

const (
	requestTimeout = 5 * time.Second
	recordPath     = "/v1/zones/{zone}/records/{name}/{type}"
)

type handler struct {
	backend recordBackend
	mux     *http.ServeMux
	slots   chan struct{}
	zone    string
	token   [sha256.Size]byte
}

func newHandler(zone, token string, records recordBackend, maxRequests int) *handler {
	handler := &handler{
		backend: records, mux: http.NewServeMux(),
		slots: make(chan struct{}, maxRequests),
		zone:  zone,
		token: sha256.Sum256([]byte(token)),
	}
	for _, operation := range recordOperations() {
		handler.mux.HandleFunc(operation.method+" "+recordPath, handler.serveRequest)
	}

	handler.mux.HandleFunc("HEAD "+recordPath, handler.methodNotAllowed)
	handler.mux.HandleFunc(recordPath, handler.methodNotAllowed)
	handler.mux.HandleFunc("/", handler.notFound)

	return handler
}

// ServeHTTP authenticates each request before resolving a record or admitting work.
func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if !handler.authenticated(r) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "a valid bearer token is required")

		return
	}

	select {
	case handler.slots <- struct{}{}:
		defer func() { <-handler.slots }()
	default:
		writeError(
			w,
			http.StatusServiceUnavailable,
			codeTooManyRequests,
			"too many active requests",
		)

		return
	}

	handler.mux.ServeHTTP(w, r)
}

func (handler *handler) authenticated(r *http.Request) bool {
	values := r.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return false
	}

	supplied := sha256.Sum256([]byte(strings.TrimPrefix(values[0], "Bearer ")))

	return subtle.ConstantTimeCompare(handler.token[:], supplied[:]) == 1
}

func (handler *handler) serveRecord(
	ctx context.Context, w http.ResponseWriter, r *http.Request, resource recordResource,
) {
	var (
		result recordSet
		err    error
	)

	switch r.Method {
	case http.MethodGet:
		result, err = handler.backend.read(ctx, resource.name, resource.rrtype)
	case http.MethodPut:
		result, err = decodeRecordSet(w, r, resource.rrtype)
		if err == nil {
			err = handler.backend.replace(ctx, resource.name, resource.rrtype, result)
		}
	case http.MethodDelete:
		err = handler.backend.delete(ctx, resource.name, resource.rrtype)
	default:
		writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "method not allowed")

		return
	}

	if err != nil {
		writeFailure(w, err)

		return
	}

	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)

		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (handler *handler) serveRequest(w http.ResponseWriter, r *http.Request) {
	input := recordInput{
		Zone: r.PathValue("zone"),
		Name: r.PathValue("name"),
		Type: r.PathValue("type"),
	}

	resource, err := input.resource(handler.zone)
	if err != nil {
		writeFailure(w, err)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	handler.serveRecord(ctx, w, r, resource)
}

func (*handler) methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "GET, PUT, DELETE")
	writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "method not allowed")
}

func (*handler) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, codeResourceNotFound, "resource not found")
}
