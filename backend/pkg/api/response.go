package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// decodeJSON decodes a request body into v, capped at 1 MiB so a
// visitor can't hand an authenticated handler an unbounded body to
// buffer into memory.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

// envelope matches the {"success", "data"|"message"|"error"} shape
// dashboard/src/lib/api.ts expects for every JWT-authenticated
// endpoint, so the frontend's existing expectations don't have to change.
type envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Success: status < 400, Data: data}); err != nil {
		log.Printf("hakaishield: encoding response: %v", err)
	}
}

func writeMessage(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Success: status < 400, Message: msg}); err != nil {
		log.Printf("hakaishield: encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Success: false, Error: msg}); err != nil {
		log.Printf("hakaishield: encoding response: %v", err)
	}
}
