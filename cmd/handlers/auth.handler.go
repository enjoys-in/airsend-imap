package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/enjoys-in/airsend-imap/cmd/services"
)

type AuthHandler struct {
	service services.AuthService
}

// NewUserHandler creates a new instance of the UserHandler with the given
// AuthService implementation. It takes an AuthService as a parameter and
// returns a new instance of the UserHandler with the given service.
func NewAuthHandler(service services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// UserLogin logs in a user based on their email address. It takes a
// http.ResponseWriter and a http.Request as parameters and returns the
// user's information if the user is found, or an error with status
// http.StatusUnauthorized if the user is not found. The user's
// information is sent as JSON in the response body.
func (h *AuthHandler) UserLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	// password := r.FormValue("password")
	user, err := h.service.GetUser(r.Context(), email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}
