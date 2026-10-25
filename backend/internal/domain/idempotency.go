package domain

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	ID           uuid.UUID  `json:"id"`
	Key          string     `json:"key"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	Endpoint     string     `json:"endpoint"`
	StatusCode   int        `json:"status_code"`
	ResponseBody string     `json:"response_body"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
}
