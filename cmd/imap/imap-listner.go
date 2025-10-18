package imap

import (
	"context"
	"crypto/tls"
	"flag"

	"log"
	"net"
	"os"
	"time"

	"github.com/ProtonMail/gluon"
	"github.com/ProtonMail/gluon/async"
	"github.com/ProtonMail/gluon/imap"

	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
	factory "github.com/enjoys-in/airsend-imap/internal/core/imap"

	"github.com/pkg/profile"
	"github.com/sirupsen/logrus"
)

var (
	cpuProfileFlag   = flag.Bool("profile-cpu", false, "Enable CPU profiling.")
	memProfileFlag   = flag.Bool("profile-mem", false, "Enable Memory profiling.")
	blockProfileFlag = flag.Bool("profile-lock", false, "Enable lock profiling.")
	profilePathFlag  = flag.String("profile-path", "", "Path where to write profile data.")
)

const (
	rollingCounterNewConnectionThreshold = 300
	rollingCounterNumberOfBuckets        = 6
	rollingCounterBucketRotationInterval = time.Second * 10
)

// var logIMAP = logrus.WithField("pkg", "server/imap") //nolint:gochecknoglobals
//
//	type IMAPEventPublisher interface {
//		PublishIMAPEvent(ctx context.Context, event imapEvents.Event)
//	}
func RunImap(app *wireframe.AppWireframe) {
	log.Println("🧩 Starting IMAP server...")

	ctx := context.Background()

	flag.Parse()
	tlsConfig, err := app.Config.LoadTLS()
	if err != nil {
		log.Printf("❌ Failed to load TLS certs: %v", err)
		return
	}
	// Initialize Gluon with your store
	dataDir := "./data/gluon_data" // Cache directory
	dbPath := "./data/gluon_state"
	if *cpuProfileFlag {
		p := profile.Start(profile.CPUProfile, profile.ProfilePath(*profilePathFlag))
		defer p.Stop()
	}

	if *memProfileFlag {
		p := profile.Start(profile.MemProfile, profile.MemProfileAllocs, profile.ProfilePath(*profilePathFlag))
		defer p.Stop()
	}

	if *blockProfileFlag {
		p := profile.Start(profile.BlockProfile, profile.ProfilePath(*profilePathFlag))
		defer p.Stop()
	}

	if level, err := logrus.ParseLevel(os.Getenv("GLUON_LOG_LEVEL")); err == nil {
		logrus.SetLevel(level)
	}
	// 	// builder := imap.NewPGStoreBuilder(app.DB.Conn)
	// 	// logIMAP.WithFields(logrus.Fields{
	// 	// 	"gluonStore": dataDir,
	// 	// 	"gluonDB":    dbPath,
	// 	// 	"version":    "df",
	// 	// }).Info("Creating IMAP server")
	// 	// var tasks *async.Group
	panicHandler := async.NoopPanicHandler{}
	// 	reporter := &reporter.NullReporter{}
	uidValidityGenerator := imap.NewEpochUIDValidityGenerator(time.Now())

	server, err := gluon.New(
		gluon.WithLogger(
			logrus.StandardLogger().WriterLevel(logrus.TraceLevel),
			logrus.StandardLogger().WriterLevel(logrus.TraceLevel),
		),
		gluon.WithDelimiter(string("/")),

		gluon.WithDataDir(dataDir),
		gluon.WithTLS(tlsConfig),
		gluon.WithDatabaseDir(dbPath),
		gluon.WithUIDValidityGenerator(uidValidityGenerator),
		gluon.WithConnectionRollingCounter(rollingCounterNewConnectionThreshold, rollingCounterNumberOfBuckets, rollingCounterBucketRotationInterval),
		gluon.WithPanicHandler(panicHandler),
	)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create server")
	}

	defer server.Close(ctx)

	log.Println("Gluon configured with:")
	log.Printf("  Data directory: %s (message cache)", dataDir)
	log.Printf("  State database: %s (IMAP state)", dbPath)
	// === Add test user ===
	instance := factory.NewConnectorFactory(app.DB.Conn, server)
	err = instance.InitializeUsers(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to add user")
		return
	}

	host := "localhost:143"
	if envHost := os.Getenv("GLUON_HOST"); envHost != "" {
		host = envHost
	}

	listener, err := net.Listen("tcp", host)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen")
	}

	logrus.Infof("Server is listening on %v", listener.Addr())

	if err := server.Serve(ctx, listener); err != nil {
		logrus.WithError(err).Fatal("Failed to serve")
	}

	for err := range server.GetErrorCh() {
		logrus.WithError(err).Error("Error while serving")
	}

	if err := listener.Close(); err != nil {
		logrus.WithError(err).Error("Failed to close listener")
	}
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
}
