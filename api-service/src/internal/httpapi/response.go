package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"api-service/internal/model"
)

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("cannot write response: %v", err)
	}
}
