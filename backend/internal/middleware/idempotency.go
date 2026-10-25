package middleware

import (
	"bytes"
	"net/http"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/repository"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

type IdempotencyMiddleware struct {
	repo repository.IdempotencyRepository
}

func NewIdempotencyMiddleware(repo repository.IdempotencyRepository) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{repo: repo}
}

func (m *IdempotencyMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		existing, err := m.repo.Get(ctx, key)
		if err == nil && existing != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(existing.StatusCode)
			_, _ = w.Write([]byte(existing.ResponseBody))
			return
		}

		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           bytes.NewBuffer(nil),
		}

		next.ServeHTTP(recorder, r)

		// Cache successful or business responses (2xx, 4xx)
		if recorder.statusCode >= 200 && recorder.statusCode < 500 {
			var userID *uuid.UUID
			if claims, ok := GetUserClaims(ctx); ok && claims != nil {
				userID = &claims.UserID
			}

			_ = m.repo.Save(ctx, &domain.IdempotencyRecord{
				ID:           uuid.New(),
				Key:          key,
				UserID:       userID,
				Endpoint:     r.Method + " " + r.URL.Path,
				StatusCode:   recorder.statusCode,
				ResponseBody: recorder.body.String(),
				CreatedAt:    time.Now().UTC(),
				ExpiresAt:    time.Now().UTC().Add(24 * time.Hour),
			})
		}
	})
}

// RequireIdempotencyKey rejects the request if the Idempotency-Key header is absent.
func RequireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			response.BadRequest(w, "Idempotency-Key basligi zorunludur", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
