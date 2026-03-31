package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/drose-drcs/signpost/internal/db"
)

// handleListSMTPUsers returns all SMTP users (password_hash excluded by json:"-").
func (s *Server) handleListSMTPUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.db.ListSMTPUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if users == nil {
		users = []db.SMTPUser{}
	}
	writeJSON(w, http.StatusOK, users)
}

// handleCreateSMTPUser creates a new SMTP user with bcrypt-hashed password.
func (s *Server) handleCreateSMTPUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := db.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := s.db.CreateSMTPUser(req.Username, hash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, fmt.Sprintf("username %q already exists", req.Username))
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-enable submission port if this is the first user
	count, _ := s.db.CountSMTPUsers()
	if count == 1 {
		s.db.SetSetting("submission_enabled", "true")
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusCreated, user)
}

// handleDeleteSMTPUser removes an SMTP user.
func (s *Server) handleDeleteSMTPUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := s.db.DeleteSMTPUser(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Auto-disable submission if no users remain
	count, _ := s.db.CountSMTPUsers()
	if count == 0 {
		s.db.SetSetting("submission_enabled", "false")
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleUpdateSMTPUserPassword updates the password for an SMTP user.
func (s *Server) handleUpdateSMTPUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := db.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := s.db.UpdateSMTPUserPassword(id, hash); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
