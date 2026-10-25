package middleware

import (
	"net/http"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/response"
)

// RequireRoles restricts access to users having one of the specified roles.
func RequireRoles(allowedRoles ...domain.UserRole) func(http.Handler) http.Handler {
	roleMap := make(map[domain.UserRole]bool)
	for _, r := range allowedRoles {
		roleMap[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetUserClaims(r.Context())
			if !ok || claims == nil {
				response.Unauthorized(w, "Yetkilendirme oturumu bulunamadi")
				return
			}

			if !roleMap[claims.Role] {
				response.Forbidden(w, "Bu kaynak icin erisim yetkiniz bulunmamaktadir")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
