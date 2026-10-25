package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type BookingHandler struct {
	svc service.BookingService
}

func NewBookingHandler(svc service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

// ==========================================
// Class Handlers
// ==========================================

func (h *BookingHandler) CreateClass(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var req service.CreateClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek formati", err.Error())
		return
	}

	class, err := h.svc.CreateClass(r.Context(), req, claims.UserID)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, class)
}

func (h *BookingHandler) ListClasses(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") != "false"
	classes, err := h.svc.ListClasses(r.Context(), activeOnly)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, classes)
}

// ==========================================
// Schedule Handlers
// ==========================================

func (h *BookingHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var req service.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek formati", err.Error())
		return
	}

	schedule, err := h.svc.CreateSchedule(r.Context(), req, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrClassNotFound) || errors.Is(err, domain.ErrUserNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, schedule)
}

func (h *BookingHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz seans ID formati", nil)
		return
	}

	schedule, err := h.svc.GetScheduleByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrScheduleNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, schedule)
}

func (h *BookingHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	var branchID *uuid.UUID
	if bStr := r.URL.Query().Get("branch_id"); bStr != "" {
		if bUUID, err := uuid.Parse(bStr); err == nil {
			branchID = &bUUID
		}
	}

	var trainerID *uuid.UUID
	if tStr := r.URL.Query().Get("trainer_id"); tStr != "" {
		if tUUID, err := uuid.Parse(tStr); err == nil {
			trainerID = &tUUID
		}
	}

	var from *time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if fTime, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &fTime
		}
	}

	var to *time.Time
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if tTime, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &tTime
		}
	}

	schedules, err := h.svc.ListSchedules(r.Context(), branchID, trainerID, from, to)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, schedules)
}

// ==========================================
// Booking & Waitlist Handlers
// ==========================================

// BookClass handles reservations or waitlisting.
func (h *BookingHandler) BookClass(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	scheduleID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz seans ID formati", nil)
		return
	}

	// Only members can book classes (or staff booking on behalf of themselves)
	targetMemberID := claims.UserID

	// If manager/reception wants to book on behalf of another member:
	if qMember := r.URL.Query().Get("member_id"); qMember != "" && (claims.Role == domain.RoleManager || claims.Role == domain.RoleReception) {
		if parsedMember, err := uuid.Parse(qMember); err == nil {
			targetMemberID = parsedMember
		}
	}

	result, err := h.svc.BookClass(r.Context(), targetMemberID, scheduleID)
	if err != nil {
		if errors.Is(err, domain.ErrScheduleNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrClassAlreadyBooked) {
			response.Conflict(w, "ERR_ALREADY_BOOKED", err.Error())
			return
		}
		if errors.Is(err, domain.ErrClassAlreadyInWaitlist) {
			response.Conflict(w, "ERR_ALREADY_IN_WAITLIST", err.Error())
			return
		}
		if errors.Is(err, domain.ErrScheduleTimeConflict) {
			response.Conflict(w, "ERR_TIME_CONFLICT", err.Error())
			return
		}
		if errors.Is(err, domain.ErrNoActiveMembership) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrBranchAccessDenied) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInsufficientEntries) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		if errors.Is(err, domain.ErrSchedulePast) || errors.Is(err, domain.ErrScheduleCancelled) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	if result.IsWaitlisted {
		response.JSON(w, http.StatusAccepted, result)
	} else {
		response.JSON(w, http.StatusCreated, result)
	}
}

// CancelBooking cancels a confirmed booking with the 2-hour rule and auto-promotion.
func (h *BookingHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	bookingID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz rezervasyon ID formati", nil)
		return
	}

	cancelResult, err := h.svc.CancelBooking(r.Context(), claims.UserID, claims.Role, bookingID)
	if err != nil {
		if errors.Is(err, domain.ErrBookingNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrBookingAlreadyCancelled) {
			response.Conflict(w, "ERR_ALREADY_CANCELLED", err.Error())
			return
		}
		if errors.Is(err, domain.ErrCannotCancelPastClass) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, cancelResult)
}

// CancelWaitlist cancels an entry from the waitlist.
func (h *BookingHandler) CancelWaitlist(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	waitlistID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz bekleme listesi ID formati", nil)
		return
	}

	err = h.svc.CancelWaitlist(r.Context(), claims.UserID, claims.Role, waitlistID)
	if err != nil {
		if errors.Is(err, domain.ErrWaitlistNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			response.Forbidden(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrWaitlistAlreadyCancelled) {
			response.Conflict(w, "ERR_WAITLIST_ALREADY_CANCELLED", err.Error())
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Bekleme listesinden basariyla ciktiniz"})
}

// MarkAttendance updates attendance (attended or no-show).
func (h *BookingHandler) MarkAttendance(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	scheduleID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz seans ID formati", nil)
		return
	}

	var req struct {
		MemberID uuid.UUID            `json:"member_id"`
		Status   domain.BookingStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek formati", err.Error())
		return
	}

	err = h.svc.MarkAttendance(r.Context(), scheduleID, req.MemberID, req.Status, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrBookingNotFound) {
			response.NotFound(w, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidAttendanceStatus) {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Yoklama durumu basariyla kaydedildi"})
}

// ListMyBookings returns the authenticated user's bookings.
func (h *BookingHandler) ListMyBookings(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	activeOnly := r.URL.Query().Get("active_only") == "true"
	bookings, err := h.svc.ListMyBookings(r.Context(), claims.UserID, activeOnly)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, bookings)
}

// ListMyWaitlist returns the authenticated user's waitlist entries.
func (h *BookingHandler) ListMyWaitlist(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	waitlists, err := h.svc.ListMyWaitlist(r.Context(), claims.UserID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, waitlists)
}

// ListScheduleBookings returns all bookings for a schedule (trainer / manager / reception).
func (h *BookingHandler) ListScheduleBookings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	scheduleID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz seans ID formati", nil)
		return
	}

	bookings, err := h.svc.ListScheduleBookings(r.Context(), scheduleID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, bookings)
}

// ListScheduleWaitlist returns the waitlist for a schedule.
func (h *BookingHandler) ListScheduleWaitlist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	scheduleID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz seans ID formati", nil)
		return
	}

	waitlists, err := h.svc.ListScheduleWaitlist(r.Context(), scheduleID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, waitlists)
}
