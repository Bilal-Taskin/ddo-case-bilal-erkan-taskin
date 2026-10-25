package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type BillingHandler struct {
	svc service.BillingService
}

func NewBillingHandler(svc service.BillingService) *BillingHandler {
	return &BillingHandler{svc: svc}
}

// CreateInvoice generates a new sequential invoice with gapless numbering (MANAGER, ACCOUNTANT).
func (h *BillingHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req service.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	if req.MemberID == uuid.Nil || req.BranchID == uuid.Nil {
		response.BadRequest(w, "member_id ve branch_id zorunludur", nil)
		return
	}

	if req.AmountKurus <= 0 {
		response.BadRequest(w, "amount_kurus 0'dan buyuk olmalidir", nil)
		return
	}

	invoice, err := h.svc.CreateSequentialInvoice(r.Context(), req)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, invoice)
}

// ListInvoices lists invoices with role filtering (Members see only their own).
func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var memberID *uuid.UUID
	if claims.Role == domain.RoleMember {
		memberID = &claims.UserID
	} else if memberStr := r.URL.Query().Get("member_id"); memberStr != "" {
		if id, err := uuid.Parse(memberStr); err == nil {
			memberID = &id
		}
	}

	var branchID *uuid.UUID
	if branchStr := r.URL.Query().Get("branch_id"); branchStr != "" {
		if id, err := uuid.Parse(branchStr); err == nil {
			branchID = &id
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

	invoices, total, err := h.svc.ListInvoices(r.Context(), branchID, memberID, limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"items":  invoices,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetInvoice returns invoice details.
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz fatura ID", nil)
		return
	}

	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	invoice, err := h.svc.GetInvoiceByID(r.Context(), id)
	if err != nil {
		response.NotFound(w, "Fatura bulunamadi")
		return
	}

	// Members can only access their own invoices
	if claims.Role == domain.RoleMember && invoice.MemberID != claims.UserID {
		response.Forbidden(w, "Baska bir uyeye ait faturaya erisemezsiniz")
		return
	}

	response.JSON(w, http.StatusOK, invoice)
}

// PayInvoice executes payment with idempotency support.
func (h *BillingHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz fatura ID", nil)
		return
	}

	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var body struct {
		CardToken      string `json:"card_token"`
		IdempotencyKey string `json:"idempotency_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	// Prefer Header Idempotency-Key if present
	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey == "" {
		idempKey = body.IdempotencyKey
	}

	req := service.ProcessPaymentRequest{
		InvoiceID:      invoiceID,
		MemberID:       claims.UserID,
		CardToken:      body.CardToken,
		IdempotencyKey: idempKey,
	}

	payment, err := h.svc.ProcessPayment(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrInvoiceAlreadyPaid) {
			response.Conflict(w, "ERR_INVOICE_ALREADY_PAID", "Fatura zaten odenmis")
			return
		}
		if errors.Is(err, domain.ErrPaymentFailed) {
			response.JSON(w, http.StatusPaymentRequired, payment)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, payment)
}

// ProcessAutoRenewals processes due recurring payments (Day 1, 3, 5 retries and Day 7 suspension).
func (h *BillingHandler) ProcessAutoRenewals(w http.ResponseWriter, r *http.Request) {
	targetDate := time.Now().UTC()
	dateStr := r.URL.Query().Get("date")
	if dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			targetDate = parsed
		}
	}

	result, err := h.svc.ProcessAutoRenewalQueue(r.Context(), targetDate)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// ListAutoRenewals lists recurring billing queue items.
func (h *BillingHandler) ListAutoRenewals(w http.ResponseWriter, r *http.Request) {
	var status *domain.AutoRenewalStatus
	statusStr := r.URL.Query().Get("status")
	if statusStr != "" {
		s := domain.AutoRenewalStatus(statusStr)
		status = &s
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

	items, total, err := h.svc.ListAutoRenewals(r.Context(), status, limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
