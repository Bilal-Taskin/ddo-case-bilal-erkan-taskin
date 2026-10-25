package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID        uuid.UUID       `json:"id"`
	UserID    *uuid.UUID      `json:"user_id,omitempty"`
	Action    string          `json:"action"`
	Entity    string          `json:"entity"`
	EntityID  string          `json:"entity_id"`
	OldValues json.RawMessage `json:"old_values,omitempty"`
	NewValues json.RawMessage `json:"new_values,omitempty"`
	IPAddress *string         `json:"ip_address,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}
