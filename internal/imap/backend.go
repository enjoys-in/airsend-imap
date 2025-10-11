package imap

import (
	"context"
	"database/sql"
	"encoding/json"

	"log"
	"strings"

	"time"

	"github.com/ProtonMail/gluon/connector"
	"github.com/ProtonMail/gluon/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/queries"
	"github.com/enjoys-in/airsend-imap/internal/utils/encryption"
)

// MyDatabaseConnector implements the connector.Connector interface
type MyDatabaseConnector struct {
	db      *sql.DB
	user    *UserConfig
	updates chan imap.Update
}
type OpenPGPKeys struct {
	PrivateKey            string `json:"privateKey"`
	PublicKey             string `json:"publicKey"`
	RevocationCertificate string `json:"revocationCertificate"`
}
type SystemEmail struct {
	IsSystemEmail    bool   `json:"is_system_email"`
	SystemEmailReply string `json:"system_email_reply"`
}
type UserConfig struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	Hash        string      `json:"hash"`
	TenantName  string      `json:"tenant_name"`
	MailboxSize int         `json:"mailbox_size"`
	Usage       int         `json:"usage"`
	Key         string      `json:"key"`
	OpenPGP     OpenPGPKeys `json:"open_pgp"`
	SystemEmail SystemEmail `json:"system_email"`
}

// Constructor
func NewMyDatabaseConnector(db *sql.DB) connector.Connector {
	return &MyDatabaseConnector{
		db:      db,
		updates: make(chan imap.Update, 100),
	}
}

// Authorize authenticates user
func (c *MyDatabaseConnector) Authorize(ctx context.Context, username string, password []byte) bool {
	var (
		id, hash, tenant, key     string
		mailboxSize, usage        int
		openPGPJSON, sysEmailJSON []byte
	)
	parts := strings.Split(username, "@")
	row := c.db.QueryRowContext(ctx, queries.GetAuthUserQuery(), username, parts[1])

	err := row.Scan(&id,
		&hash,
		&tenant,
		&mailboxSize,
		&usage,
		&key,
		&openPGPJSON,
		&sysEmailJSON)

	if err != nil {
		log.Fatal("Scanning Rows", err)
		return false
	}

	isMatch, err := encryption.ValidatePassword(hash, string(password))

	if err != nil || !isMatch {
		log.Fatal("Password Mismatch", err)

		return false
	}
	// Unmarshal JSON columns
	var openPGP OpenPGPKeys
	if len(openPGPJSON) > 0 {
		if err := json.Unmarshal(openPGPJSON, &openPGP); err != nil {
			log.Printf("⚠️ Failed to parse OpenPGP JSON for %s: %v", username, err)
		}
	}

	var sysEmail SystemEmail
	if len(sysEmailJSON) > 0 {
		if err := json.Unmarshal(sysEmailJSON, &sysEmail); err != nil {
			log.Printf("⚠️ Failed to parse SystemEmail JSON for %s: %v", username, err)
		}
	}

	user := &UserConfig{
		ID:          id,
		Email:       username,
		Hash:        hash,
		TenantName:  tenant,
		MailboxSize: mailboxSize,
		Usage:       usage,
		Key:         key,
		OpenPGP:     openPGP,
		SystemEmail: sysEmail,
	}
	c.user = user

	return true
}

// CreateMailbox creates mailbox
func (c *MyDatabaseConnector) CreateMailbox(ctx context.Context, name []string) (imap.Mailbox, error) {
	mboxName := name[len(name)-1]
	var mboxID string
	err := c.db.QueryRowContext(ctx,
		"INSERT INTO mailboxes (user_id, name, created_at) VALUES ($1,$2,$3) RETURNING id",
		c.user.Email, mboxName, time.Now(),
	).Scan(&mboxID)
	if err != nil {
		return imap.Mailbox{}, err
	}

	mbox := imap.Mailbox{
		ID:   imap.MailboxID(mboxID),
		Name: name,
	}
	c.updates <- &imap.MailboxCreated{Mailbox: mbox}
	return mbox, nil
}

// GetMessageLiteral fetches raw content
func (c *MyDatabaseConnector) GetMessageLiteral(ctx context.Context, id imap.MessageID) ([]byte, error) {
	var literal []byte
	err := c.db.QueryRowContext(ctx, "SELECT raw_content FROM messages WHERE id=$1 AND user_id=$2",
		string(id), c.user.Email).Scan(&literal)
	return literal, err
}

// GetMailboxVisibility returns visibility
func (c *MyDatabaseConnector) GetMailboxVisibility(ctx context.Context, mboxID imap.MailboxID) imap.MailboxVisibility {
	return imap.Visible
}

// UpdateMailboxName updates mailbox
func (c *MyDatabaseConnector) UpdateMailboxName(ctx context.Context, mboxID imap.MailboxID, newName []string) error {
	_, err := c.db.ExecContext(ctx, "UPDATE mailboxes SET name=$1 WHERE id=$2 AND user_id=$3",
		newName[len(newName)-1], string(mboxID), c.user.Email)
	return err
}

// DeleteMailbox deletes mailbox
func (c *MyDatabaseConnector) DeleteMailbox(ctx context.Context, mboxID imap.MailboxID) error {
	_, err := c.db.ExecContext(ctx, "DELETE FROM mailboxes WHERE id=$1 AND user_id=$2",
		string(mboxID), c.user.Email)
	return err
}

// CreateMessage inserts message
func (c *MyDatabaseConnector) CreateMessage(ctx context.Context, mboxID imap.MailboxID, literal []byte, flags imap.FlagSet, date time.Time) (imap.Message, []byte, error) {
	var msgID string
	err := c.db.QueryRowContext(ctx,
		"INSERT INTO messages (user_id, mailbox_id, raw_content, seen, flagged, date, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
		c.user.Email, string(mboxID), literal, flags.Contains(imap.FlagSeen), flags.Contains(imap.FlagFlagged), date, time.Now(),
	).Scan(&msgID)
	if err != nil {
		return imap.Message{}, nil, err
	}
	msg := imap.Message{ID: imap.MessageID(msgID), Flags: flags, Date: date}
	return msg, literal, nil
}

// AddMessagesToMailbox associates messages
func (c *MyDatabaseConnector) AddMessagesToMailbox(ctx context.Context, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	for _, msgID := range messageIDs {
		_, err := c.db.ExecContext(ctx, "INSERT INTO message_mailbox (message_id, mailbox_id) VALUES ($1,$2)",
			string(msgID), string(mboxID))
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveMessagesFromMailbox dissociates messages
func (c *MyDatabaseConnector) RemoveMessagesFromMailbox(ctx context.Context, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	for _, msgID := range messageIDs {
		_, err := c.db.ExecContext(ctx, "DELETE FROM message_mailbox WHERE message_id=$1 AND mailbox_id=$2",
			string(msgID), string(mboxID))
		if err != nil {
			return err
		}
	}
	return nil
}

// MoveMessages moves messages atomically
func (c *MyDatabaseConnector) MoveMessages(ctx context.Context, messageIDs []imap.MessageID, mboxFromID, mboxToID imap.MailboxID) (bool, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	for _, msgID := range messageIDs {
		_, err := tx.ExecContext(ctx,
			"UPDATE messages SET mailbox_id=$1 WHERE id=$2 AND mailbox_id=$3 AND user_id=$4",
			string(mboxToID), string(msgID), string(mboxFromID), c.user.Email)
		if err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

// MarkMessagesSeen flags seen
func (c *MyDatabaseConnector) MarkMessagesSeen(ctx context.Context, messageIDs []imap.MessageID, seen bool) error {
	for _, msgID := range messageIDs {
		_, err := c.db.ExecContext(ctx, "UPDATE messages SET seen=$1 WHERE id=$2 AND user_id=$3",
			seen, string(msgID), c.user.Email)
		if err != nil {
			return err
		}
	}
	return nil
}

// MarkMessagesFlagged flags messages
func (c *MyDatabaseConnector) MarkMessagesFlagged(ctx context.Context, messageIDs []imap.MessageID, flagged bool) error {
	for _, msgID := range messageIDs {
		_, err := c.db.ExecContext(ctx, "UPDATE messages SET flagged=$1 WHERE id=$2 AND user_id=$3",
			flagged, string(msgID), c.user.Email)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListMailboxes lists all mailboxes for the authenticated user
func (c *MyDatabaseConnector) ListMailboxes(ctx context.Context) ([]imap.Mailbox, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT id, name FROM mailboxes WHERE user_id=$1", c.user.Email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mailboxes []imap.Mailbox
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		mbox := imap.Mailbox{
			ID:   imap.MailboxID(id),
			Name: []string{name},
		}
		mailboxes = append(mailboxes, mbox)
	}
	return mailboxes, nil
}

// ListMessages lists messages in a mailbox, optionally filtered by sequence numbers
func (c *MyDatabaseConnector) ListMessages(ctx context.Context, mboxID imap.MailboxID, seqset *imap.SeqSet) ([]imap.Message, error) {
	query := "SELECT id, seen, flagged, date FROM messages WHERE mailbox_id=$1 AND user_id=$2 ORDER BY date ASC"
	rows, err := c.db.QueryContext(ctx, query, string(mboxID), c.user.Email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []imap.Message
	seqNum := uint32(1) // IMAP sequence numbers start at 1

	for rows.Next() {
		var id string
		var seen, flagged bool
		var date time.Time
		if err := rows.Scan(&id, &seen, &flagged, &date); err != nil {
			return nil, err
		}

		flags := imap.FlagSet{}
		if seen {
			flags.Add(imap.FlagSeen)
		}
		if flagged {
			flags.Add(imap.FlagFlagged)
		}

		// If seqset is provided, only include messages in it
		if seqset != nil {
			seqNum++
			continue
		}

		messages = append(messages, imap.Message{
			ID:    imap.MessageID(id),
			Flags: flags,
			Date:  date,
		})
		seqNum++
	}

	return messages, nil
}

// GetMessageFlags returns flags for a set of message IDs
func (c *MyDatabaseConnector) GetMessageFlags(ctx context.Context, messageIDs []imap.MessageID) (map[imap.MessageID]imap.FlagSet, error) {
	flagsMap := make(map[imap.MessageID]imap.FlagSet)

	for _, msgID := range messageIDs {
		var seen, flagged bool
		err := c.db.QueryRowContext(ctx, "SELECT seen, flagged FROM messages WHERE id=$1 AND user_id=$2",
			string(msgID), c.user.Email).Scan(&seen, &flagged)
		if err != nil {
			return nil, err
		}

		flags := imap.FlagSet{}
		if seen {
			flags.Add(imap.FlagSeen)
		}
		if flagged {
			flags.Add(imap.FlagFlagged)
		}

		flagsMap[msgID] = flags
	}

	return flagsMap, nil
}

// GetUpdates returns channel
func (c *MyDatabaseConnector) GetUpdates() <-chan imap.Update {
	return c.updates
}

// Close closes connector
func (c *MyDatabaseConnector) Close(ctx context.Context) error {
	close(c.updates)
	return nil
}
