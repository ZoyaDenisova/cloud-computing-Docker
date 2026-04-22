package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingMiddlewareCallsNext(t *testing.T) {
	t.Parallel()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	})

	handler := LoggingMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Fatalf("expected wrapped handler to be called")
	}
	if recorder.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, recorder.Code)
	}
}
