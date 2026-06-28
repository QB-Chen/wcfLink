package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIAuthExemptsHealth(t *testing.T) {
	t.Parallel()

	s := &Server{apiToken: "secret"}
	handler := s.withAPIAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected exempt health endpoint, got status %d", resp.Code)
	}
}

func TestAPIAuthRejectsMissingToken(t *testing.T) {
	t.Parallel()

	s := &Server{apiToken: "secret"}
	handler := s.withAPIAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got status %d", resp.Code)
	}
}

func TestAPIAuthAcceptsBearerToken(t *testing.T) {
	t.Parallel()

	s := &Server{apiToken: "secret"}
	handler := s.withAPIAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected authorized request, got status %d", resp.Code)
	}
}
