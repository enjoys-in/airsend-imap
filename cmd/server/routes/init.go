package routes

import (
	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
	"net/http"
)

func InitRoutes(app *wireframe.AppWireframe) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", app.Handler.AuthHandler.UserLogin)

	// mux.HandleFunc("/api/imap/users/add", api.authMiddleware(api.handleAddUser))
	// mux.HandleFunc("/api/imap/users/add-batch", api.authMiddleware(api.handleAddUserBatch))
	// mux.HandleFunc("/api/imap/users/remove", api.authMiddleware(api.handleRemoveUser))
	// mux.HandleFunc("/api/imap/users/list", api.authMiddleware(api.handleListUsers))
	// mux.HandleFunc("/api/imap/users/check", api.authMiddleware(api.handleCheckUser))

	// // Public endpoint (no auth)
	// mux.HandleFunc("/api/imap/status", api.handleStatus)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return mux
}
