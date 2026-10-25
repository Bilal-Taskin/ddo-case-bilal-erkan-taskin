package pagination

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"
)

type PageRequest struct {
	Limit  int
	Cursor string
}

type PageResponse struct {
	Items      interface{} `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
	HasMore    bool        `json:"has_more"`
}

func ParsePageRequest(r *http.Request, defaultLimit, maxLimit int) PageRequest {
	limitStr := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	cursor := r.URL.Query().Get("cursor")

	return PageRequest{
		Limit:  limit,
		Cursor: cursor,
	}
}

func EncodeCursor(t time.Time, id string) string {
	raw := t.Format(time.RFC3339Nano) + "," + id
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	bytes, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := splitCursor(string(bytes))
	if len(parts) != 2 {
		return time.Time{}, "", err
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return t, parts[1], nil
}

func splitCursor(s string) []string {
	var res []string
	idx := 0
	for i, c := range s {
		if c == ',' {
			res = append(res, s[idx:i])
			res = append(res, s[i+1:])
			return res
		}
	}
	return res
}
