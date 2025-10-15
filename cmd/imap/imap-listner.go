package imap

import (
	"context"
	"crypto/tls"
	"flag"

	"github.com/ProtonMail/gluon"
	"github.com/ProtonMail/gluon/async"
	imapEvents "github.com/ProtonMail/gluon/events"
	"github.com/ProtonMail/gluon/limits"
	"github.com/ProtonMail/gluon/reporter"
	"github.com/pkg/profile"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enjoys-in/airsend-imap/cmd/wireframe"
	"github.com/enjoys-in/airsend-imap/internal/core/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/imap/connector"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

var logIMAP = logrus.WithField("pkg", "server/imap") //nolint:gochecknoglobals
type IMAPEventPublisher interface {
	PublishIMAPEvent(ctx context.Context, event imapEvents.Event)
}

const (
	rollingCounterNewConnectionThreshold = 300
	rollingCounterNumberOfBuckets        = 6
	rollingCounterBucketRotationInterval = time.Second * 10
)

var (
	cpuProfileFlag   = flag.Bool("profile-cpu", false, "Enable CPU profiling.")
	memProfileFlag   = flag.Bool("profile-mem", false, "Enable Memory profiling.")
	blockProfileFlag = flag.Bool("profile-lock", false, "Enable lock profiling.")
	profilePathFlag  = flag.String("profile-path", "", "Path where to write profile data.")
)

func RunImap(app *wireframe.AppWireframe) {
	log.Println("🧩 Starting IMAP server...")
	flag.Parse()
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
	// logIMAP.WithFields(logrus.Fields{
	// 	"gluonStore": dataDir,
	// 	"gluonDB":    dbPath,
	// 	"version":    "df",
	// }).Info("Creating IMAP server")
	// var tasks *async.Group
	panicHandler := async.NoopPanicHandler{}
	reporter := &reporter.NullReporter{}
	uidValidityGenerator := imap.NewUIDValidityGenerator()
	// reporter   = reporter.NewContextWithReporter(logIMAP)
	server, err := gluon.New(
		gluon.WithDelimiter(string("/")),
		gluon.WithTLS(tlsConfig),
		gluon.WithIMAPLimits(limits.IMAP{}),
		gluon.WithDataDir(""),
		gluon.WithDatabaseDir(""),
		gluon.WithStoreBuilder(builder),
		gluon.WithUIDValidityGenerator(uidValidityGenerator),
		gluon.WithConnectionRollingCounter(rollingCounterNewConnectionThreshold, rollingCounterNumberOfBuckets, rollingCounterBucketRotationInterval),
		getGluonVersionInfo(),
		gluon.WithReporter(reporter),
		gluon.WithPanicHandler(panicHandler),
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
	// tasks.Once(func(ctx context.Context) {
	// 	watcher := server.AddWatcher()
	// 	for {
	// 		select {
	// 		case <-ctx.Done():
	// 			return
	// 		case e, ok := <-watcher:
	// 			if !ok {
	// 				return
	// 			}

	// 			eventPublisher.PublishIMAPEvent(ctx, e)
	// 		}
	// 	}
	// })

	// tasks.Once(func(ctx context.Context) {
	// 	async.RangeContext(ctx, server.GetErrorCh(), func(err error) {
	// 		logIMAP.WithError(err).Error("IMAP server error")
	// 	})
	// })
	// Block until shutdown
	<-ctx.Done()

}
func getGluonVersionInfo() gluon.Option {
	return gluon.WithVersionInfo(
		int(1), //nolint:gosec // disable G115
		int(0), //nolint:gosec // disable G115
		int(0), //nolint:gosec // disable G115
		"constants.FullAppName",
		"TODO",
		"TODO",
	)
}
