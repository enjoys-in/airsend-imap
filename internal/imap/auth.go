package imap

import (
	"context"
	"database/sql"
	"errors"
)

// PGAuthenticator implements gluon.Authenticator
type PGAuthenticator struct {
	db *sql.DB
}

// NewPGAuthenticator creates a new instance
func NewPGAuthenticator(db *sql.DB) *PGAuthenticator {
	return &PGAuthenticator{db: db}
}

// Authenticate checks email/password and returns userID (email) if valid
func (a *PGAuthenticator) Authenticate(ctx context.Context, username, password string) (string, error) {
	var hashedPassword string
	var email string

	err := a.db.QueryRowContext(ctx, `
        SELECT email, password
        FROM mail_accounts
        WHERE email = $1
    `, username).Scan(&email, &hashedPassword)

	if err == sql.ErrNoRows {
		return "", errors.New("invalid credentials")
	} else if err != nil {
		return "", err
	}

	// Return email as userID
	return email, nil
}
