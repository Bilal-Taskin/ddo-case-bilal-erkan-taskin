package middleware

import (
	"context"
	"net/http"
	"strings"

	"fitgiris/internal/platform/response"
	"fitgiris/internal/platform/security"
)

type contextKey string

const UserClaimsContextKey contextKey = "user_claims"

type AuthMiddleware struct {
	jwtManager *security.JWTManager
}

func NewAuthMiddleware(jwtManager *security.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{jwtManager: jwtManager}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Unauthorized(w, "Yetkilendirme basligi (Authorization) eksik")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(w, "Gecersiz Authorization formati. 'Bearer <token>' bekleniyor")
			return
		}

		tokenString := parts[1]
		claims, err := m.jwtManager.ValidateToken(tokenString)
		if err != nil {
			response.Unauthorized(w, "Gecersiz veya suresi dolmus token")
			return
		}

		if claims.TokenType != "access" {
			response.Unauthorized(w, "Gecersiz token tipi, access token bekleniyor")
			return
		}

		ctx := context.WithValue(r.Context(), UserClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserClaims(ctx context.Context) (*security.Claims, bool) {
	claims, ok := ctx.Value(UserClaimsContextKey).(*security.Claims)
	return claims, ok
}
