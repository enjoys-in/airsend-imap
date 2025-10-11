package services

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/enjoys-in/airsend-imap/internal/core/imap"
	"golang.org/x/time/rate"
)

type APIServer struct {
	cf      *imap.ConnectorFactory
	apiKey  string // Simple API key for authentication
	limiter *rate.Limiter
}

// NewAPIServer creates a new API server instance, given a connector factory and an API key.
// The API server is used to authenticate and authorize API requests.
func NewAPIServer(cf *imap.ConnectorFactory, apiKey string) *APIServer {
	return &APIServer{
		cf:      cf,
		apiKey:  apiKey,
		limiter: rate.NewLimiter(10, 50), // 10 req/sec, burst of 50
	}
}
func (api *APIServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey != api.apiKey {
			http.Error(w, `{"error":"unauthorized","message":"Invalid or missing API key"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (api *APIServer) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !api.limiter.Allow() {
			http.Error(w, `{"error":"rate_limit_exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// POST /api/imap/users/add
// Body: {"email": "user@example.com"}
func (api *APIServer) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, `{"error":"validation_error","message":"email is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gluonUserID, err := api.cf.GetOrCreateUser(ctx, req.Email, "password")
	if err != nil {
		log.Printf("API: Failed to add user %s: %v", req.Email, err)
		http.Error(w, `{"error":"user_add_failed","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User added to IMAP server",
		"email":   req.Email,
		"id":      gluonUserID,
	})
}

// POST /api/imap/users/add-batch
// Body: {"emails": ["user1@example.com", "user2@example.com"]}
func (api *APIServer) handleAddUserBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Emails []string `json:"emails"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}

	results := make([]map[string]interface{}, 0)
	successCount := 0

	for _, email := range req.Emails {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		gluonUserID, err := api.cf.GetOrCreateUser(ctx, email, "password")
		cancel()

		result := map[string]interface{}{
			"email": email,
			"id":    gluonUserID,
		}

		if err != nil {
			result["success"] = false
			result["error"] = err.Error()
		} else {
			result["success"] = true
			successCount++
		}

		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   len(req.Emails),
		"added":   successCount,
		"failed":  len(req.Emails) - successCount,
		"results": results,
	})
}

// DELETE /api/imap/users/remove
// Body: {"email": "user@example.com"}
func (api *APIServer) handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := api.cf.RemoveUser(ctx, req.Email)
	if err != nil {
		http.Error(w, `{"error":"user_remove_failed","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User removed from IMAP server",
		"email":   req.Email,
	})
}

// GET /api/imap/users/list
func (api *APIServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, count := api.cf.GetActiveUserCount()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   count,
		"users":   users,
	})
}

// GET /api/imap/users/check?email=user@example.com
func (api *APIServer) handleCheckUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, `{"error":"validation_error","message":"email parameter required"}`, http.StatusBadRequest)
		return
	}

	isLoaded := api.cf.IsUserLoaded(email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"email":   email,
		"loaded":  isLoaded,
	})
}

// GET /api/imap/status
func (api *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, count := api.cf.GetActiveUserCount()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"status":       "running",
		"active_users": count,
		"timestamp":    time.Now().Unix(),
	})
}
