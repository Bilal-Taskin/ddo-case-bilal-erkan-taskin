package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
)

type mockIdempotencyRepo struct {
	mu    sync.Mutex
	store map[string]*domain.IdempotencyRecord
}

func newMockIdempotencyRepo() *mockIdempotencyRepo {
	return &mockIdempotencyRepo{store: make(map[string]*domain.IdempotencyRecord)}
}

func (m *mockIdempotencyRepo) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store[key], nil
}

func (m *mockIdempotencyRepo) Save(ctx context.Context, record *domain.IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[record.Key] = record
	return nil
}

func TestIdempotencyMiddleware(t *testing.T) {
	repo := newMockIdempotencyRepo()
	idempotencyMW := middleware.NewIdempotencyMiddleware(repo)

	var handlerCalls int32

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&handlerCalls, 1)
		response.JSON(w, http.StatusCreated, map[string]string{"message": "islem tamamlandi"})
	})

	chained := idempotencyMW.Handle(testHandler)

	key := "test-idempotency-key-12345"

	// 1st Call
	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req1.Header.Set("Idempotency-Key", key)
	rr1 := httptest.NewRecorder()
	chained.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("1. istek basarisiz: status %d", rr1.Code)
	}
	if atomic.LoadInt32(&handlerCalls) != 1 {
		t.Fatalf("Handler 1 kez cagrilmaliydi, alinan: %d", handlerCalls)
	}

	// 2nd Call with identical Idempotency-Key
	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.Header.Set("Idempotency-Key", key)
	rr2 := httptest.NewRecorder()
	chained.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Fatalf("2. istek basarisiz: status %d", rr2.Code)
	}
	if rr2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("2. istek onbellekten donmeliydi, X-Cache header yok")
	}
	if atomic.LoadInt32(&handlerCalls) != 1 {
		t.Fatalf("Handler tekrar cagrilmamaliydi! Toplam cagri: %d", handlerCalls)
	}
	if rr1.Body.String() != rr2.Body.String() {
		t.Errorf("1. ve 2. yanit govdesi ayni olmaliydi:\n1: %s\n2: %s", rr1.Body.String(), rr2.Body.String())
	}
}
