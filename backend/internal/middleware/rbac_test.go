package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/platform/security"
)

func setupTestRouter(jwtManager *security.JWTManager) http.Handler {
	r := chi.NewRouter()
	authMW := middleware.NewAuthMiddleware(jwtManager)

	r.Group(func(r chi.Router) {
		r.Use(authMW.Authenticate)

		// Manager only route
		r.With(middleware.RequireRoles(domain.RoleManager)).Get("/manager-only", func(w http.ResponseWriter, r *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"result": "success"})
		})

		// Trainer or Manager route
		r.With(middleware.RequireRoles(domain.RoleTrainer, domain.RoleManager)).Get("/trainer-manager", func(w http.ResponseWriter, r *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"result": "success"})
		})
	})

	return r
}

func TestRBAC_NoToken_Returns401(t *testing.T) {
	jwtManager := security.NewJWTManager("test-secret-32-chars-long-fitgiris!")
	router := setupTestRouter(jwtManager)

	req := httptest.NewRequest(http.MethodGet, "/manager-only", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Beklenen HTTP 401, alinan: %d", rr.Code)
	}

	var errResp response.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("JSON parse hatasi: %v", err)
	}

	if errResp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("Beklenen hata kodu 'UNAUTHORIZED', alinan: %s", errResp.Error.Code)
	}
}

func TestRBAC_WrongRole_Returns403(t *testing.T) {
	jwtManager := security.NewJWTManager("test-secret-32-chars-long-fitgiris!")
	router := setupTestRouter(jwtManager)

	memberUser := &domain.User{
		ID:        uuid.New(),
		Email:     "member@fitgiris.com",
		Role:      domain.RoleMember,
		FirstName: "Can",
		LastName:  "Demir",
		IsActive:  true,
	}

	tokens, err := jwtManager.GenerateTokenPair(memberUser)
	if err != nil {
		t.Fatalf("Token üretilemedi: %v", err)
	}

	// Member tries to access Manager route
	req := httptest.NewRequest(http.MethodGet, "/manager-only", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Beklenen HTTP 403, alinan: %d", rr.Code)
	}

	var errResp response.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("JSON parse hatasi: %v", err)
	}

	if errResp.Error.Code != "FORBIDDEN" {
		t.Errorf("Beklenen hata kodu 'FORBIDDEN', alinan: %s", errResp.Error.Code)
	}
}

func TestRBAC_CorrectRole_Returns200(t *testing.T) {
	jwtManager := security.NewJWTManager("test-secret-32-chars-long-fitgiris!")
	router := setupTestRouter(jwtManager)

	managerUser := &domain.User{
		ID:        uuid.New(),
		Email:     "manager@fitgiris.com",
		Role:      domain.RoleManager,
		FirstName: "Zeynep",
		LastName:  "Kaya",
		IsActive:  true,
	}

	tokens, err := jwtManager.GenerateTokenPair(managerUser)
	if err != nil {
		t.Fatalf("Token üretilemedi: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/manager-only", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Beklenen HTTP 200, alinan: %d, body: %s", rr.Code, rr.Body.String())
	}
}
