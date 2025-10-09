package imap

import (
	"log"
	"net"

	"github.com/emersion/go-imap/server"
	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
)

func RunImap(app *wireframe.AppWireframe) {
	log.Println("📡 Starting IMAP server...")

	be := NewBackend(app.Service)

	s := server.New(be)
	s.Addr = ":993" // IMAPS
	s.AllowInsecureAuth = false

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatalf("❌ Failed to start IMAP listener: %v", err)
	}

	log.Println("📬 IMAP server listening on", s.Addr)
	if err := s.Serve(listener); err != nil {
		log.Fatalf("❌ IMAP server stopped: %v", err)
	}
}
