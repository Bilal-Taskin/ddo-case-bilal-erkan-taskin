package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input service.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	ip := r.RemoteAddr
	user, tokens, err := h.authService.Register(r.Context(), input, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			response.Conflict(w, "USER_EXISTS", err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidRole) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	if req.Email == "" || req.Password == "" {
		response.BadRequest(w, "E-posta ve parola zorunludur", nil)
		return
	}

	ip := r.RemoteAddr
	user, tokens, err := h.authService.Login(r.Context(), req.Email, req.Password, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPassword) {
			response.Unauthorized(w, "Gecersiz e-posta veya parola")
			return
		}
		if errors.Is(err, domain.ErrInactiveUser) {
			response.Forbidden(w, "Kullanici hesabi askiya alinmis veya aktif degil")
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	if req.RefreshToken == "" {
		response.BadRequest(w, "Refresh token zorunludur", nil)
		return
	}

	tokens, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(w, "Gecersiz veya suresi dolmus refresh token")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "Oturum bilgisi bulunamadi")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		response.NotFound(w, "Kullanici profili bulunamadi")
		return
	}

	response.JSON(w, http.StatusOK, user)
}
