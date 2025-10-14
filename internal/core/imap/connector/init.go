package connector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/ProtonMail/gluon"
	"github.com/enjoys-in/airsend-imap/internal/core/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/queries"
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

	// Create new connector for this user
	gluonID, err := cf.LoadGluonIdFromDB(ctx, email)

	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("❌ Failed to load user from DB: %v", err)
		}
	}

	userConnector := imap.NewMyDatabaseConnector(cf.db)
	// If gluonID exists, try loading user into Gluon
	if gluonID != "" {
		if exists, err := cf.server.LoadUser(ctx, userConnector, gluonID, []byte(password)); err == nil && !exists {
			log.Printf("✅ Loaded existing Gluon user: %s (%s)", email, gluonID)
			return gluonID, nil
		}
	} else {
		// Add user to Gluon
		gluonUserID, err := cf.server.AddUser(ctx, userConnector, []byte(password))
		if err != nil {
			return "", fmt.Errorf("failed to add user to Gluon: %w", err)
		}
		if err := cf.saveGluonIDToDB(ctx, email, gluonUserID); err != nil {
			log.Printf("⚠️ Failed to save new Gluon ID for %s: %v", email, err)
		}
		cf.userConnectors[email] = gluonUserID
		gluonID = gluonUserID
		log.Printf("Dynamically added user: %s (Gluon ID: %s)", email, gluonUserID)
	}

	return gluonID, nil
}

func (cf *ConnectorFactory) saveGluonIDToDB(ctx context.Context, email, gluonID string) error {
	_, err := cf.db.ExecContext(ctx, `UPDATE mail_accounts SET gluon_id = $1 WHERE email = $2;`,
		gluonID, email)
	return err
}

func (cf *ConnectorFactory) LoadGluonIdFromDB(ctx context.Context, email string) (string, error) {
	cf.mu.RLock()
	if gluonUserID, exists := cf.userConnectors[email]; exists {
		cf.mu.RUnlock()
		return gluonUserID, nil
	}
	cf.mu.RUnlock()

	row := cf.db.QueryRowContext(ctx, queries.GetGluonIDQuery(), email)

	var gluonUserID string
	err := row.Scan(&gluonUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("⚠️ No user found in DB for email: %s", email)
			return "", err // or return "", sql.ErrNoRows if you want caller to handle
		}
		log.Printf("❌ Failed to scan user row for %s: %v", email, err)
		return "", err
	}

	// Handle empty gluon_id value
	if strings.TrimSpace(gluonUserID) == "" {
		log.Printf("⚠️ Gluon ID empty for user: %s", email)
		return "", fmt.Errorf("empty gluon_id for email %s", email)
	}

	// Cache and return
	cf.mu.Lock()
	cf.userConnectors[email] = gluonUserID
	cf.mu.Unlock()

	log.Printf("→ Loaded IMAP user from DB: %s (gluon_id: %s)", email, gluonUserID)
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
