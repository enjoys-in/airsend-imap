package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enjoys-in/airsend-imap/cmd/imap"
	api "github.com/enjoys-in/airsend-imap/cmd/server"
	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
)

// main is the entry point of the program, responsible for starting the IMAP and HTTP APIs
// in parallel and gracefully shutting down on Ctrl+C or SIGTERM.
func main() {
	app := wireframe.InitWireframe()
	defer app.DB.Close()

	// Run IMAP and HTTP in parallel
	go imap.RunImap(app)
	go api.RunHttpApi(app)

	time.Sleep(2 * time.Second)
	// Graceful shutdown
	log.Println("🧩 Services started. Press Ctrl+C to stop.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("🛑 Shutting down gracefully...")
}
