package httpapi

import (
	"encoding/json"
	"net/http"
)

type jsonErrorResponse struct {
	Error jsonError `json:"error"`
}

type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	setNoSniff(w)
	setNoStore(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, jsonErrorResponse{
		Error: jsonError{
			Code:    code,
			Message: message,
		},
	})
}
