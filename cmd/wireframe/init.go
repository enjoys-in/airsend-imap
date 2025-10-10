package wireframe

import (
	"log"

	"github.com/enjoys-in/airsend-imap/cmd/handlers"
	"github.com/enjoys-in/airsend-imap/cmd/repository"
	"github.com/enjoys-in/airsend-imap/cmd/services"
	"github.com/enjoys-in/airsend-imap/config"
	plugins "github.com/enjoys-in/airsend-imap/internal/plugins/postgres"
)

type AppWireframe struct {
	Config     config.Config
	DB         *plugins.DB
	Repository *repository.Repository
	Service    *services.Services
	Handler    *handlers.Handlers
}

// InitWireframe initializes the application by creating a DB connection,
// a repository, a service and a handler. It returns an App struct
// containing all the necessary components. If the DB connection fails,
// it logs an error and exits.
func InitWireframe() *AppWireframe {
	cfg := config.GetConfig()
	db, err := plugins.CreateDBConnection(cfg.DB.DBHost, cfg.DB.DBPort, cfg.DB.DBUser, cfg.DB.DBPassword, cfg.DB.DBName, cfg.DB.DBSSLMode)
	if err != nil {
		log.Fatal("❌ Failed to connect DB:", err)
	}
	defer db.Close()
	err = db.Conn.Ping()
	if err != nil {
		log.Fatal("❌ Failed to ping DB:", err)
	}
	log.Println("✅ DB connected")

	repo := repository.NewRepository(db)
	svc := services.NewServices(repo)
	h := handlers.NewHandlers(svc)
	return &AppWireframe{
		Config:     cfg,
		DB:         db,
		Repository: repo,
		Service:    svc,
		Handler:    h,
	}
}

func (w *AppWireframe) Close() {
	if w.DB != nil {
		_ = w.DB.Close()
	}
}
