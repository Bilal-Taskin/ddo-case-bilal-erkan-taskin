package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type MembershipHandler struct {
	svc *service.MembershipService
}

func NewMembershipHandler(svc *service.MembershipService) *MembershipHandler {
	return &MembershipHandler{svc: svc}
}

// Package Types Handlers

func (h *MembershipHandler) CreatePackageType(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePackageTypeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	pkgType, err := h.svc.CreatePackageType(r.Context(), input)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, pkgType)
}

func (h *MembershipHandler) ListPackageTypes(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") != "false"
	list, err := h.svc.ListPackageTypes(r.Context(), activeOnly)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *MembershipHandler) GetPackageType(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz paket tipi kimligi", nil)
		return
	}

	pt, err := h.svc.GetPackageType(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrPackageTypeNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, pt)
}

// Membership Operations Handlers

func (h *MembershipHandler) Purchase(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var input service.PurchasePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	// If member role and member_id is omitted, default to own ID
	if input.MemberID == uuid.Nil {
		input.MemberID = claims.UserID
	}

	ip := r.RemoteAddr
	pkg, err := h.svc.PurchasePackage(r.Context(), input, claims.UserID, claims.Role, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrPackageTypeNotFound) || errors.Is(err, domain.ErrUserNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrPackageTypeInactive) || errors.Is(err, domain.ErrInactiveUser) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, pkg)
}

func (h *MembershipHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var memberID *uuid.UUID
	// Members can only see their own memberships
	if claims.Role == domain.RoleMember {
		memberID = &claims.UserID
	} else {
		// Staff can filter by member_id
		if qMember := r.URL.Query().Get("member_id"); qMember != "" {
			if parsed, err := uuid.Parse(qMember); err == nil {
				memberID = &parsed
			}
		}
	}

	var status *domain.MembershipStatus
	if qStatus := r.URL.Query().Get("status"); qStatus != "" {
		st := domain.MembershipStatus(qStatus)
		status = &st
	}

	limit := 20
	if qLimit := r.URL.Query().Get("limit"); qLimit != "" {
		if val, err := strconv.Atoi(qLimit); err == nil && val > 0 {
			limit = val
		}
	}

	offset := 0
	if qOffset := r.URL.Query().Get("offset"); qOffset != "" {
		if val, err := strconv.Atoi(qOffset); err == nil && val >= 0 {
			offset = val
		}
	}

	list, total, err := h.svc.ListMemberships(r.Context(), memberID, status, limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"items":  list,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *MembershipHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	pkg, err := h.svc.GetMembership(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	// Security: member can only inspect own membership
	if claims.Role == domain.RoleMember && pkg.MemberID != claims.UserID {
		response.Forbidden(w, "")
		return
	}

	response.JSON(w, http.StatusOK, pkg)
}

type FreezeRequest struct {
	Days   int    `json:"days"`
	Reason string `json:"reason"`
}

func (h *MembershipHandler) Freeze(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	var req FreezeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	ip := r.RemoteAddr
	pkg, err := h.svc.FreezeMembership(r.Context(), service.FreezeMembershipInput{
		MemberPackageID: id,
		Days:            req.Days,
		Reason:          req.Reason,
	}, claims.UserID, claims.Role, &ip)

	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		if errors.Is(err, domain.ErrFreezeLimitExceeded) {
			response.Conflict(w, domain.ErrCodeFreezeLimitExceeded, err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, pkg)
}

func (h *MembershipHandler) Unfreeze(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	ip := r.RemoteAddr
	pkg, err := h.svc.UnfreezeMembership(r.Context(), id, claims.UserID, claims.Role, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) || errors.Is(err, domain.ErrNoActiveFreeze) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, pkg)
}

type TransferRequest struct {
	ToMemberID uuid.UUID `json:"to_member_id"`
	Reason     string    `json:"reason"`
}

func (h *MembershipHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	ip := r.RemoteAddr
	newPkg, err := h.svc.TransferMembership(r.Context(), service.TransferMembershipInput{
		MemberPackageID: id,
		ToMemberID:      req.ToMemberID,
		Reason:          req.Reason,
	}, claims.UserID, claims.Role, &ip)

	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) || errors.Is(err, domain.ErrUserNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		if errors.Is(err, domain.ErrCannotTransferToSelf) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		if errors.Is(err, domain.ErrMembershipExpired) || errors.Is(err, domain.ErrInsufficientEntries) {
			response.Conflict(w, "MEMBERSHIP_DEPLETED", err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, newPkg)
}

func (h *MembershipHandler) UpgradeQuote(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	newPkgTypeStr := r.URL.Query().Get("new_package_type_id")
	newPkgTypeID, err := uuid.Parse(newPkgTypeStr)
	if err != nil {
		response.BadRequest(w, "new_package_type_id parametresi gereklidir", nil)
		return
	}

	quote, err := h.svc.GetUpgradeQuote(r.Context(), id, newPkgTypeID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) || errors.Is(err, domain.ErrPackageTypeNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrSamePackageTypeUpgrade) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		if errors.Is(err, domain.ErrMembershipNotActive) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, quote)
}

type UpgradeRequest struct {
	NewPackageTypeID uuid.UUID `json:"new_package_type_id"`
}

func (h *MembershipHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	var req UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek govdesi", err.Error())
		return
	}

	ip := r.RemoteAddr
	newPkg, quote, err := h.svc.UpgradeMembership(r.Context(), id, req.NewPackageTypeID, claims.UserID, claims.Role, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) || errors.Is(err, domain.ErrPackageTypeNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		if errors.Is(err, domain.ErrSamePackageTypeUpgrade) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"new_membership": newPkg,
		"upgrade_quote":  quote,
	})
}

type CancelRequest struct {
	Reason string `json:"reason"`
}

func (h *MembershipHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz uyelik kimligi", nil)
		return
	}

	var req CancelRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ip := r.RemoteAddr
	err = h.svc.CancelMembership(r.Context(), id, req.Reason, claims.UserID, claims.Role, &ip)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			response.Conflict(w, domain.ErrCodeInvalidStateTransition, err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Uyelik basariyla iptal edildi"})
}
