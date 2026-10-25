package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"fitgiris/internal/platform/response"
	"fitgiris/internal/service"
)

type ReportHandler struct {
	svc service.ReportService
}

func NewReportHandler(svc service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) RevenueReport(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	branchStr := r.URL.Query().Get("branch_id")

	var from, to time.Time
	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		to, _ = time.Parse("2006-01-02", toStr)
		// set to end of day
		to = to.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	var branchID *uuid.UUID
	if branchStr != "" {
		if id, err := uuid.Parse(branchStr); err == nil {
			branchID = &id
		}
	}

	rep, err := h.svc.GetRevenueReport(r.Context(), from, to, branchID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, rep)
}

func (h *ReportHandler) OccupancyReport(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	branchStr := r.URL.Query().Get("branch_id")

	var date time.Time
	if dateStr != "" {
		date, _ = time.Parse("2006-01-02", dateStr)
	} else {
		date = time.Now()
	}

	var branchID *uuid.UUID
	if branchStr != "" {
		if id, err := uuid.Parse(branchStr); err == nil {
			branchID = &id
		}
	}

	rep, err := h.svc.GetHourlyOccupancy(r.Context(), branchID, date)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, rep)
}

func (h *ReportHandler) TrainerReport(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to time.Time
	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		to, _ = time.Parse("2006-01-02", toStr)
		to = to.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	rep, err := h.svc.GetTrainerReport(r.Context(), from, to)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, rep)
}
