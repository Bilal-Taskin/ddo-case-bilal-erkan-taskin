package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fitgiris/internal/handler"
)

func TestSwaggerHandler(t *testing.T) {
	sh := handler.NewSwaggerHandler()

	t.Run("ServeOpenAPI", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
		rec := httptest.NewRecorder()

		sh.ServeOpenAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/yaml") {
			t.Fatalf("expected application/yaml content-type, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), "FitGiriş API Dokümantasyonu") {
			t.Fatalf("expected body to contain openapi spec title")
		}
	})

	t.Run("ServeSwaggerUI", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
		rec := httptest.NewRecorder()

		sh.ServeSwaggerUI(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("expected text/html content-type, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
			t.Fatalf("expected body to contain SwaggerUI bundle script")
		}
	})
}
