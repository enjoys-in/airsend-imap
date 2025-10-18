package connector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"

	"time"

	"github.com/ProtonMail/gluon/connector"
	"github.com/ProtonMail/gluon/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/queries"
	user "github.com/enjoys-in/airsend-imap/internal/interfaces/user"
	"github.com/enjoys-in/airsend-imap/internal/utils/encryption"
)

var (
	ErrNoSuchMailbox = errors.New("no such mailbox")
	ErrNoSuchMessage = errors.New("no such message")

	ErrInvalidPrefix   = errors.New("invalid prefix")
	ErrRenameForbidden = errors.New("rename operation is not allowed")
	ErrDeleteForbidden = errors.New("delete operation is not allowed")
)
var defaultFlags imap.FlagSet = imap.NewFlagSet(
	imap.FlagSeen,
	imap.FlagAnswered,
	imap.FlagFlagged,
	imap.FlagDeleted,
	imap.FlagDraft,
	"$Important",
	"$Pinned",
	"$Archived",
)

type MyDBConnector struct {
	db                         *sql.DB
	email                      string
	updates                    chan imap.Update
	state                      *MailboxState
	user                       *user.UserConfig
	lastClientIMAPID           imap.IMAPID
	allowUnknownMailbox        bool
	folderPrefix, labelsPrefix string
	updatesAllowedToFail       int32
	queueLock                  sync.Mutex
	queue                      []imap.Update
	mailboxVisibilities        map[imap.MailboxID]imap.MailboxVisibility
}

func NewConnector(db *sql.DB, email string) *MyDBConnector {
	return &MyDBConnector{
		db:                  db,
		email:               email,
		updates:             make(chan imap.Update, 100),
		state:               newMailboxState(defaultFlags, defaultFlags, imap.FlagSet{}),
		user:                nil,
		lastClientIMAPID:    imap.NewIMAPID(),
		allowUnknownMailbox: true,
		folderPrefix:        "",
		labelsPrefix:        "",
		mailboxVisibilities: make(map[imap.MailboxID]imap.MailboxVisibility),
	}
}

func (c *MyDBConnector) Init(ctx context.Context, cache connector.IMAPState) error { return nil }

// Authorize returns whether the given username/password combination are valid for this connector.
func (c *MyDBConnector) Authorize(ctx context.Context, username string, password []byte) bool {
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
		log.Print("Scanning Rows", err)
		return false
	}

	isMatch := encryption.ValidatePassword(hash, string(password))

	if !isMatch {
		log.Print("Password Mismatch", err)
		return false
	}
	// Unmarshal JSON columns
	var openPGP user.OpenPGPKeys
	if len(openPGPJSON) > 0 {
		if err := json.Unmarshal(openPGPJSON, &openPGP); err != nil {
			log.Printf("⚠️ Failed to parse OpenPGP JSON for %s: %v", username, err)
		}
	}

	var sysEmail user.SystemEmail
	if len(sysEmailJSON) > 0 {
		if err := json.Unmarshal(sysEmailJSON, &sysEmail); err != nil {
			log.Printf("⚠️ Failed to parse SystemEmail JSON for %s: %v", username, err)
		}
	}

	user := &user.UserConfig{
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

// CreateMailbox creates a mailbox with the given name.
func (c *MyDBConnector) CreateMailbox(ctx context.Context, cache connector.IMAPStateWrite, name []string) (imap.Mailbox, error) {

	return imap.Mailbox{}, nil
}

// GetMessageLiteral is intended to be used by Gluon when, for some reason, the local cached data no longer exists.
// Note: this can get called from different go routines.
func (c *MyDBConnector) GetMessageLiteral(ctx context.Context, id imap.MessageID) ([]byte, error) {
	return []byte("fdffd"), nil
}

// GetMailboxVisibility can be used to retrieve the visibility of mailboxes for connected clients.
func (c *MyDBConnector) GetMailboxVisibility(ctx context.Context, mboxID imap.MailboxID) imap.MailboxVisibility {

	return imap.Visible
}

// UpdateMailboxName sets the name of the mailbox with the given ID.
func (c *MyDBConnector) UpdateMailboxName(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID, newName []string) error {
	return nil
}

// DeleteMailbox deletes the mailbox with the given ID.
func (c *MyDBConnector) DeleteMailbox(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID) error {
	return nil
}

// CreateMessage creates a new message on the remote.
func (c *MyDBConnector) CreateMessage(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID, literal []byte, flags imap.FlagSet, date time.Time) (imap.Message, []byte, error) {
	return imap.Message{}, nil, nil
}

// AddMessagesToMailbox adds the given messages to the given mailbox.
func (c *MyDBConnector) AddMessagesToMailbox(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	return nil
}

// RemoveMessagesFromMailbox removes the given messages from the given mailbox.
func (c *MyDBConnector) RemoveMessagesFromMailbox(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	return nil
}

// MoveMessages removes the given messages from one mailbox and adds them to the another mailbox.
// Returns true if the original messages should be removed from mboxFromID (e.g: Distinguishing between labels and folders).
func (c *MyDBConnector) MoveMessages(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxFromID, mboxToID imap.MailboxID) (bool, error) {
	return true, nil
}

// MarkMessagesSeen sets the seen value of the given messages.
func (c *MyDBConnector) MarkMessagesSeen(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, seen bool) error {
	return nil
}

// MarkMessagesFlagged sets the flagged value of the given messages.
func (c *MyDBConnector) MarkMessagesFlagged(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, flagged bool) error {
	return nil
}

// MarkMessagesForwarded sets the forwarded value of the give messages.
func (c *MyDBConnector) MarkMessagesForwarded(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, forwarded bool) error {
	return nil
}

// GetUpdates returns a stream of updates that the gluon server should apply.
func (c *MyDBConnector) GetUpdates() <-chan imap.Update {
	return c.updates
}

func (c *MyDBConnector) Close(ctx context.Context) error {
	close(c.updates)
	return nil
}
