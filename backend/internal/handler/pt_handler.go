package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/middleware"
	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type PTHandler struct {
	svc service.PTService
}

func NewPTHandler(svc service.PTService) *PTHandler {
	return &PTHandler{svc: svc}
}

type PurchasePTPackageRequest struct {
	TrainerID     uuid.UUID `json:"trainer_id"`
	TotalSessions int       `json:"total_sessions"`
	PriceKurus    int64     `json:"price_kurus"`
}

type BookPTAppointmentRequest struct {
	PTPackageID uuid.UUID `json:"pt_package_id"`
	BranchID    uuid.UUID `json:"branch_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Notes       string    `json:"notes"`
}

func (h *PTHandler) PurchasePackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var req PurchasePTPackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek formati", err.Error())
		return
	}

	pkg, err := h.svc.PurchasePackage(r.Context(), claims.UserID, req.TrainerID, req.TotalSessions, req.PriceKurus)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, pkg)
}

func (h *PTHandler) ListPackages(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	if claims.Role == domain.RoleTrainer {
		packages, err := h.svc.ListTrainerPackages(r.Context(), claims.UserID)
		if err != nil {
			response.InternalError(w, err.Error())
			return
		}
		response.JSON(w, http.StatusOK, packages)
		return
	}

	packages, err := h.svc.ListMemberPackages(r.Context(), claims.UserID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, packages)
}

func (h *PTHandler) BookAppointment(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	var req BookPTAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Gecersiz istek formati", err.Error())
		return
	}

	app, err := h.svc.BookAppointment(r.Context(), claims.UserID, req.PTPackageID, req.BranchID, req.StartTime, req.EndTime, req.Notes)
	if err != nil {
		if err == domain.ErrPTTrainerConflict {
			response.Conflict(w, "Egitmenin bu saatte baska bir randevusu mevcuttur", "ERR_PT_TRAINER_CONFLICT")
			return
		}
		if err == domain.ErrPTNoSessionsRemaining {
			response.Conflict(w, "Paketinizde kalan seans bulunmamaktadir", "ERR_PT_NO_SESSIONS_REMAINING")
			return
		}
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusCreated, app)
}

func (h *PTHandler) ListAppointments(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	if claims.Role == domain.RoleTrainer {
		apps, err := h.svc.ListTrainerAppointments(r.Context(), claims.UserID, nil, nil)
		if err != nil {
			response.InternalError(w, err.Error())
			return
		}
		response.JSON(w, http.StatusOK, apps)
		return
	}

	apps, err := h.svc.ListMemberAppointments(r.Context(), claims.UserID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, apps)
}

func (h *PTHandler) CancelAppointment(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetUserClaims(r.Context())
	if !ok || claims == nil {
		response.Unauthorized(w, "")
		return
	}

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz randevu ID", nil)
		return
	}

	if err := h.svc.CancelAppointment(r.Context(), claims.UserID, appID); err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Randevu basariyla iptal edildi, seans paketine iade edildi"})
}

func (h *PTHandler) CompleteAppointment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Gecersiz randevu ID", nil)
		return
	}

	if err := h.svc.CompleteAppointment(r.Context(), appID); err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Randevu tamamlandi olarak isaretlendi"})
}
