package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fitgiris/internal/handler"
)

func TestHealthCheckDegradedWithoutDB(t *testing.T) {
	h := handler.NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.HealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Beklenen HTTP status %d, alinan %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp handler.HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse hatasi: %v", err)
	}

	if resp.Status != "DEGRADED" {
		t.Errorf("Beklenen status 'DEGRADED', alinan '%s'", resp.Status)
	}

	if resp.Database != "DOWN" {
		t.Errorf("Beklenen database 'DOWN', alinan '%s'", resp.Database)
	}
}
