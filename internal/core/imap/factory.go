package imap

import (
	"context"
	"database/sql"

	"fmt"
	"log"

	"github.com/ProtonMail/gluon"
	"github.com/enjoys-in/airsend-imap/internal/core/imap/connector"
	_ "github.com/enjoys-in/airsend-imap/internal/core/imap/connector"
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

var defaultPass = []byte("default_password")

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
func (cf *ConnectorFactory) InitializeUsers(ctx context.Context) error {

	users, err := cf.GetAllUsersWithImapEnabled(ctx)

	if err != nil {
		return fmt.Errorf("failed to get users with IMAP enabled: %w", err)
	}
	for _, u := range users {
		log.Printf("→ Initializing IMAP user: %s", u.EmailID)
		gid, err := cf.GetOrCreateUser(ctx, u.EmailID, u.GluonID)
		if err != nil {
			log.Printf("❌ Failed to initialize IMAP user %s: %v", u.EmailID, err)
			continue
		}
		log.Printf("✅ IMAP user initialized: %s (Gluon ID: %s)", u.EmailID, gid)
	}
	return nil
}
func (cf *ConnectorFactory) GetOrCreateUser(ctx context.Context, email string, gluon_id *string) (string, error) {
	// Check if user is already loaded
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	var gluonUserID string

	userConnector := connector.NewConnector(cf.db, email)
	// If gluonID is provided, use it; otherwise, try loading from DB
	if gluon_id == nil {
		gluonUserID, err := cf.server.AddUser(ctx, userConnector, defaultPass)
		if err != nil {
			return "", fmt.Errorf("failed to add user to Gluon: %w", err)
		}
		if err := cf.saveGluonIDToDB(ctx, email, gluonUserID); err != nil {
			log.Printf("⚠️ Failed to save new Gluon ID for %s: %v", email, err)
		}

		gluonUserID = gluonUserID
		log.Printf("Dynamically added user: %s (Gluon ID: %s)", email, gluonUserID)
	} else {
		gluonUserID := *gluon_id
		if exists, err := cf.server.LoadUser(ctx, userConnector, gluonUserID, defaultPass); err == nil && !exists {
			log.Printf("✅ Loaded existing Gluon user: %s (%s)", email, gluonUserID)
		}

	}

	cf.userConnectors[email] = gluonUserID
	if err := userConnector.Sync(ctx); err != nil {
		fmt.Printf("❌ Failed to sync user %s: %v", email, err)
		return "", fmt.Errorf("failed to sync user %s: %w", email, err)
	}
	log.Printf("→ IMAP user ready: %s (Gluon ID: %s)", email, gluonUserID)
	return gluonUserID, nil
}
func (cf *ConnectorFactory) saveGluonIDToDB(ctx context.Context, email, gluonID string) error {
	_, err := cf.db.ExecContext(ctx, `UPDATE mail_accounts SET gluon_id = $1 WHERE email = $2;`,
		gluonID, email)
	return err
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
func (cf *ConnectorFactory) GetAllUsersWithImapEnabled(ctx context.Context) ([]struct {
	GluonID *string
	EmailID string
}, error) {
	rows, err := cf.db.QueryContext(ctx, queries.GetAllUserWithImapEnabled())
	if err != nil {
		log.Printf("❌ Failed to query users with IMAP enabled: %v", err)
		return nil, err
	}
	defer rows.Close()

	var users []struct {
		GluonID *string
		EmailID string
	}

	for rows.Next() {
		var (
			gluon_id sql.NullString
			email_id string
		)
		if err := rows.Scan(&gluon_id, &email_id); err != nil {
			log.Printf("❌ Failed to scan user row: %v", err)
			continue
		}
		var gIDPtr *string
		if gluon_id.Valid {
			gIDPtr = &gluon_id.String
		}
		users = append(users, struct {
			GluonID *string
			EmailID string
		}{
			GluonID: gIDPtr,
			EmailID: email_id,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
