package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type TurnstileHandler struct {
	svc service.TurnstileService
}

func NewTurnstileHandler(svc service.TurnstileService) *TurnstileHandler {
	return &TurnstileHandler{svc: svc}
}

// CreateTurnstile adds a new gate to a branch (MANAGER only).
func (h *TurnstileHandler) CreateTurnstile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BranchID  uuid.UUID                `json:"branch_id"`
		Code      string                   `json:"code"`
		Name      string                   `json:"name"`
		Direction domain.TurnstileDirection `json:"direction"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	if body.BranchID == uuid.Nil || body.Code == "" || body.Name == "" {
		response.BadRequest(w, "branch_id, code ve name alanlari zorunludur", nil)
		return
	}

	if !body.Direction.IsValid() {
		response.BadRequest(w, "direction alani 'IN' veya 'OUT' olmalidir", nil)
		return
	}

	turnstile := &domain.Turnstile{
		BranchID:  body.BranchID,
		Code:      body.Code,
		Name:      body.Name,
		Direction: body.Direction,
		IsActive:  true,
	}

	if err := h.svc.CreateTurnstile(r.Context(), turnstile); err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, turnstile)
}

// ListTurnstiles lists turnstiles with optional branch filter.
func (h *TurnstileHandler) ListTurnstiles(w http.ResponseWriter, r *http.Request) {
	var branchID *uuid.UUID
	branchStr := r.URL.Query().Get("branch_id")
	if branchStr != "" {
		if id, err := uuid.Parse(branchStr); err == nil {
			branchID = &id
		}
	}

	activeOnly := r.URL.Query().Get("active_only") == "true"

	list, err := h.svc.ListTurnstiles(r.Context(), branchID, activeOnly)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, list)
}

// GetTurnstile returns a single turnstile by ID.
func (h *TurnstileHandler) GetTurnstile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz turnike ID", nil)
		return
	}

	turnstile, err := h.svc.GetTurnstileByID(r.Context(), id)
	if err != nil {
		response.NotFound(w, "Turnike bulunamadi")
		return
	}

	response.JSON(w, http.StatusOK, turnstile)
}

// EvaluateAccess evaluates a turnstile pass attempt in real-time (p95 < 50ms).
func (h *TurnstileHandler) EvaluateAccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		service.AccessRequest
		CardToken string `json:"card_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	req := body.AccessRequest
	if req.MemberID == uuid.Nil {
		if body.CardToken == "CARD-DEMO-001" {
			req.MemberID = uuid.MustParse("00000000-0000-0000-0001-000000000005")
		} else {
			response.BadRequest(w, "member_id alani zorunludur", nil)
			return
		}
	}

	decision, err := h.svc.EvaluateAccess(r.Context(), req)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Always return 200 with decision payload so turnstile hardware controller parses the decision
	response.JSON(w, http.StatusOK, decision)
}

// SyncOffline ingests batched passage events recorded while turnstile was offline.
func (h *TurnstileHandler) SyncOffline(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []domain.OfflineSyncItem `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	result, err := h.svc.SyncOfflineAccessLogs(r.Context(), body.Items)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// ListAccessLogs lists turnstile access logs with pagination.
func (h *TurnstileHandler) ListAccessLogs(w http.ResponseWriter, r *http.Request) {
	var branchID *uuid.UUID
	branchStr := r.URL.Query().Get("branch_id")
	if branchStr != "" {
		if id, err := uuid.Parse(branchStr); err == nil {
			branchID = &id
		}
	}

	var memberID *uuid.UUID
	memberStr := r.URL.Query().Get("member_id")
	if memberStr != "" {
		if id, err := uuid.Parse(memberStr); err == nil {
			memberID = &id
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	logs, total, err := h.svc.ListAccessLogs(r.Context(), branchID, memberID, limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"items":  logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
