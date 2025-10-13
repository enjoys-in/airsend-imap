package imap

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ProtonMail/gluon"
	"github.com/ProtonMail/gluon/limits"
	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
	"github.com/enjoys-in/airsend-imap/internal/core/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/imap/connector"
	_ "github.com/mattn/go-sqlite3"
)

func RunImap(app *wireframe.AppWireframe) {
	log.Println("🧩 Starting IMAP server...")

	// Load TLS configuration
	tlsConfig, err := app.Config.LoadTLS()
	if err != nil {
		log.Printf("❌ Failed to load TLS certs: %v", err)
		return
	}
	// Initialize Gluon with your store
	dataDir := "./data/gluon_data" // Cache directory
	dbPath := "./data/gluon_state"
	builder := imap.NewPGStoreBuilder(app.DB.Conn)

	server, err := gluon.New(
		gluon.WithTLS(tlsConfig),
		gluon.WithIMAPLimits(limits.IMAP{}),
		gluon.WithDataDir(dataDir),
		gluon.WithDatabaseDir(dbPath),
		gluon.WithStoreBuilder(builder),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Gluon configured with:")
	log.Printf("  Data directory: %s (message cache)", dataDir)
	log.Printf("  State database: %s (IMAP state)", dbPath)

	// Context with graceful shutdown support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("Stopping IMAP server gracefully...")
		cancel()
	}()
	factory := connector.NewConnectorFactory(app.DB.Conn, server)

	gluonUserID, err := factory.GetOrCreateUser(ctx, "mullayam06@airsend.in", "12345678")
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}
	log.Printf("Created/retrieved user with Gluon ID: %s", gluonUserID)
	listener, err := net.Listen("tcp", "0.0.0.0:143")
	if err != nil {
		log.Printf("❌ Failed to listen on IMAP (1143): %v", err)
		return
	}
	log.Println("📬 IMAP server listening on 0.0.0.0:143")
	// === Start Plain IMAP (Port 143) ===
	go func() {
		if err := server.Serve(ctx, listener); err != nil && err != context.Canceled {
			log.Printf("❌ IMAP server error: %v", err)
		}
	}()
	// Give Gluon time to start
	time.Sleep(1 * time.Second)

	// === Start IMAPS (TLS) on Port 993 ===
	go func() {

		tlsListener, err := tls.Listen("tcp", "0.0.0.0:993", tlsConfig)
		if err != nil {
			log.Printf("❌ Failed to listen on IMAPS (993): %v", err)
			return
		}

		log.Println("🔒 IMAPS (TLS) server listening on 0.0.0.0:993")

		if err := server.Serve(ctx, tlsListener); err != nil && err != context.Canceled {
			log.Printf("❌ IMAPS server error: %v", err)
		}
	}()

	// Block until shutdown
	<-ctx.Done()

}
