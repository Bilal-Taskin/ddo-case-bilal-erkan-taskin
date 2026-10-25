package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Database  string    `json:"database"`
	Timestamp time.Time `json:"timestamp"`
	UptimeSec int64     `json:"uptime_sec"`
}

type HealthHandler struct {
	pool      *pgxpool.Pool
	startTime time.Time
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{
		pool:      pool,
		startTime: time.Now(),
	}
}

func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "UP"
	httpStatus := http.StatusOK

	if h.pool == nil {
		dbStatus = "DOWN"
		httpStatus = http.StatusServiceUnavailable
	} else if err := h.pool.Ping(ctx); err != nil {
		dbStatus = "DOWN"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := HealthResponse{
		Status:    "UP",
		Database:  dbStatus,
		Timestamp: time.Now().UTC(),
		UptimeSec: int64(time.Since(h.startTime).Seconds()),
	}

	if dbStatus == "DOWN" {
		resp.Status = "DEGRADED"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}
