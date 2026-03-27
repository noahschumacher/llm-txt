package server

import (
	"encoding/json"
	"net/http"
)

type passwordCheckRequest struct {
	Password string `json:"password"`
}

func (s *Server) handlePasswordCheck(w http.ResponseWriter, r *http.Request) {
	var req passwordCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Password != s.cfg.Password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}
