package api

import (
	"github.com/enjoys-in/airsend-imap/cmd/server/routes"
	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
	"log"
	"net/http"
)

// RunHttpApis sets up an HTTP API server with a single endpoint at /login
// for user login. It uses the AuthHandler.UserLogin function to handle the
// request. The server is started on port 8080 and runs until
// interrupted by Ctrl+C or SIGTERM. If the server stops due to an
// error, it logs the error and exits.
func RunHttpApi(app *wireframe.AppWireframe) {

	defer app.DB.Close()

	mux := routes.InitRoutes(app)

	port := ":8080"
	log.Println("🚀 HTTP Server Initializing on port", port)
	server := &http.Server{
		Addr:    port,
		Handler: mux,
	}

	log.Println("🚀 HTTP API running on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("❌ HTTP server failed: %v", err)
	}
}
