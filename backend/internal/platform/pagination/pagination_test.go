package pagination_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fitgiris/internal/platform/pagination"
)

func TestCursorEncodeDecode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	id := "550e8400-e29b-41d4-a716-446655440000"

	cursor := pagination.EncodeCursor(now, id)
	if cursor == "" {
		t.Fatal("Cursor bos olamaz")
	}

	decodedTime, decodedID, err := pagination.DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("Cursor decode edilemedi: %v", err)
	}

	if !decodedTime.Equal(now) {
		t.Errorf("Beklenen zaman %v, alinan %v", now, decodedTime)
	}

	if decodedID != id {
		t.Errorf("Beklenen ID %s, alinan %s", id, decodedID)
	}
}

func TestParsePageRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items?limit=25&cursor=abc123xyz", nil)
	pageReq := pagination.ParsePageRequest(req, 10, 100)

	if pageReq.Limit != 25 {
		t.Errorf("Beklenen limit 25, alinan %d", pageReq.Limit)
	}
	if pageReq.Cursor != "abc123xyz" {
		t.Errorf("Beklenen cursor 'abc123xyz', alinan '%s'", pageReq.Cursor)
	}

	// Exceeding max limit
	reqMax := httptest.NewRequest(http.MethodGet, "/items?limit=999", nil)
	pageReqMax := pagination.ParsePageRequest(reqMax, 10, 100)
	if pageReqMax.Limit != 100 {
		t.Errorf("Maksimum limit 100 olmaliydi, alinan %d", pageReqMax.Limit)
	}
}
