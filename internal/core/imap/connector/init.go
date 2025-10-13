package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/ProtonMail/gluon"
	"github.com/enjoys-in/airsend-imap/internal/core/imap"
	"golang.org/x/time/rate"

	"sync"
)

type ConnectorFactory struct {
	db             *sql.DB
	server         *gluon.Server
	userConnectors map[string]string // email -> gluonUserID
	mu             sync.RWMutex
}
type APIServer struct {
	cf      *ConnectorFactory
	apiKey  string // Simple API key for authentication
	limiter *rate.Limiter
}

// NewAPIServer creates a new API server instance, given a connector factory and an API key.
// The API server is used to authenticate and authorize API requests.

func NewAPIServer(cf *ConnectorFactory, apiKey string) *APIServer {
	return &APIServer{
		cf:      cf,
		apiKey:  apiKey,
		limiter: rate.NewLimiter(10, 50), // 10 req/sec, burst of 50
	}
}

func NewConnectorFactory(db *sql.DB, server *gluon.Server) *ConnectorFactory {
	return &ConnectorFactory{
		db:             db,
		server:         server,
		userConnectors: make(map[string]string),
	}
}

// GetOrCreateUser returns the Gluon user ID for an email, creating if needed
func (cf *ConnectorFactory) GetOrCreateUser(ctx context.Context, email string, password string) (string, error) {
	cf.mu.RLock()
	if gluonUserID, exists := cf.userConnectors[email]; exists {
		cf.mu.RUnlock()
		return gluonUserID, nil
	}
	cf.mu.RUnlock()
	cf.mu.Lock()
	defer cf.mu.Unlock()

	// Double-check after acquiring write lock
	if gluonUserID, exists := cf.userConnectors[email]; exists {
		return gluonUserID, nil
	}

	// Create new connector for this user
	userConnector := imap.NewMyDatabaseConnector(cf.db)

	// Add user to Gluon
	gluonUserID, err := cf.server.AddUser(ctx, userConnector, []byte(password))
	if err != nil {
		return "", fmt.Errorf("failed to add user to Gluon: %w", err)
	}

	cf.userConnectors[email] = gluonUserID
	log.Printf("Dynamically added user: %s (Gluon ID: %s)", email, gluonUserID)

	return gluonUserID, nil
}

func (cf *ConnectorFactory) RemoveUser(ctx context.Context, email string) error {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	gluonUserID, exists := cf.userConnectors[email]
	if !exists {
		return fmt.Errorf("user not loaded")
	}

	// Remove from Gluon (if API exists)
	cf.server.RemoveUser(ctx, gluonUserID, true)

	delete(cf.userConnectors, email)
	log.Printf("→ Removed IMAP user: %s", email)

	return nil
}

func (cf *ConnectorFactory) GetActiveUserCount() ([]string, int) {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	users := make([]string, 0, len(cf.userConnectors))
	for email := range cf.userConnectors {
		users = append(users, email)
	}
	return users, len(cf.userConnectors)
}

func (cf *ConnectorFactory) IsUserLoaded(email string) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	_, exists := cf.userConnectors[email]
	return exists
}
