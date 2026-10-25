package response

import (
	"encoding/json"
	"net/http"
)

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type DataResponse struct {
	Data interface{} `json:"data"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(DataResponse{Data: data})
}

func RawJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Error(w http.ResponseWriter, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func BadRequest(w http.ResponseWriter, message string, details interface{}) {
	Error(w, http.StatusBadRequest, "BAD_REQUEST", message, details)
}

func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Kimlik dogrulamasi basarisiz veya oturum suresi doldu"
	}
	Error(w, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Bu islem icin yetkiniz bulunmamaktadir"
	}
	Error(w, http.StatusForbidden, "FORBIDDEN", message, nil)
}

func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Istenen kaynak bulunamadi"
	}
	Error(w, http.StatusNotFound, "NOT_FOUND", message, nil)
}

func Conflict(w http.ResponseWriter, code, message string) {
	if code == "" {
		code = "CONFLICT"
	}
	Error(w, http.StatusConflict, code, message, nil)
}

func InternalError(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Sunucu tarafinda beklenmeyen bir hata olustu"
	}
	Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message, nil)
}
