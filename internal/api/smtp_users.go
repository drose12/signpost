package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/drose-drcs/signpost/internal/crypto"
	"github.com/drose-drcs/signpost/internal/db"
)

// handleListSMTPUsers returns all SMTP users with decrypted passwords.
func (s *Server) handleListSMTPUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.db.ListSMTPUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type userResponse struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Password  string `json:"password,omitempty"`
		Active    bool   `json:"active"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		ur := userResponse{
			ID:        u.ID,
			Username:  u.Username,
			Active:    u.Active,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if u.PasswordEnc != nil && u.PasswordNonce != nil && s.encKey != nil {
			if pw, err := crypto.Decrypt(s.encKey, *u.PasswordEnc, *u.PasswordNonce); err == nil {
				ur.Password = pw
			}
		}
		resp = append(resp, ur)
	}
	writeJSON(w, http.StatusOK, resp)
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

	// Encrypt password for display
	var passEnc, passNonce *string
	if s.encKey != nil {
		enc, nonce, err := crypto.Encrypt(s.encKey, req.Password)
		if err == nil {
			passEnc = &enc
			passNonce = &nonce
		}
	}

	user, err := s.db.CreateSMTPUser(req.Username, hash, passEnc, passNonce)
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

	// Checkpoint WAL so Maddy's auth.pass_table can read the new user
	s.db.Checkpoint()

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

	s.db.Checkpoint()
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

	// Encrypt password for display
	var passEnc, passNonce *string
	if s.encKey != nil {
		enc, nonce, err := crypto.Encrypt(s.encKey, req.Password)
		if err == nil {
			passEnc = &enc
			passNonce = &nonce
		}
	}

	if err := s.db.UpdateSMTPUserPassword(id, hash, passEnc, passNonce); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.db.Checkpoint()
	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
