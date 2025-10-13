package imap

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/ProtonMail/gluon/imap"
	"github.com/ProtonMail/gluon/store"
)

// PGStoreBuilder implements store.Builder interface
type PGStoreBuilder struct {
	db *sql.DB
}

// NewPGStoreBuilder creates a Postgres-backed store builder
func NewPGStoreBuilder(db *sql.DB) store.Builder {
	return &PGStoreBuilder{db: db}
}

// New creates a Store for a specific user
func (b *PGStoreBuilder) New(dir, userID string, passphrase []byte) (store.Store, error) {
	return &PGMailStore{
		db:     b.db,
		userID: userID,
	}, nil
}

// Delete cleans up mail for a given user
func (b *PGStoreBuilder) Delete(dir, userID string) error {
	_, err := b.db.Exec(`DELETE FROM mails_data WHERE email = $1`, userID)
	return err
}

//
// ──────────────────────────────────────────────────────────────
//   MAIL STORE IMPLEMENTATION
// ──────────────────────────────────────────────────────────────
//

// PGMailStore implements store.Store interface
type PGMailStore struct {
	db     *sql.DB
	userID string
}

// List returns all message IDs for this user
func (s *PGMailStore) List() ([]imap.InternalMessageID, error) {
	rows, err := s.db.Query(`SELECT message_id FROM mails_data WHERE email = $1`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []imap.InternalMessageID
	for rows.Next() {
		var mid string
		if err := rows.Scan(&mid); err != nil {
			return nil, err
		}

		ids = append(ids, imap.NewInternalMessageID())
	}
	return ids, nil
}

// Get retrieves raw message data by ID
func (s *PGMailStore) Get(messageID imap.InternalMessageID) ([]byte, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT content FROM mails_data WHERE email = $1 AND message_id = $2`, s.userID, messageID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	return raw, err
}

// Set stores or updates a message
func (s *PGMailStore) Set(messageID imap.InternalMessageID, reader io.Reader) error {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO mail_data (account_id, message_id, raw)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, message_id) DO UPDATE
		SET raw = EXCLUDED.raw
	`, s.userID, messageID, raw)

	return err
}

// Delete removes messages by ID
func (s *PGMailStore) Delete(messageIDs ...imap.InternalMessageID) error {
	for _, mid := range messageIDs {
		if _, err := s.db.Exec(`DELETE FROM mails_data WHERE email = $1 AND message_id = $2`, s.userID, mid); err != nil {
			return err
		}
	}
	return nil
}

// Close does nothing (for interface compliance)
func (s *PGMailStore) Close() error {
	return nil
}
