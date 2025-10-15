package wireframe

import (
	"log"

	"github.com/enjoys-in/airsend-imap/config"
	"github.com/enjoys-in/airsend-imap/internal/core/api/handlers"
	"github.com/enjoys-in/airsend-imap/internal/core/api/repository"
	"github.com/enjoys-in/airsend-imap/internal/core/api/services"
	plugins "github.com/enjoys-in/airsend-imap/internal/plugins/postgres"
)

type AppWireframe struct {
	Config     config.Config
	DB         *plugins.DB
	Repository *repository.Repository
	Service    *services.ConcreteServices
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
