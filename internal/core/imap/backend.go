package imap

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"log"
	"strings"

	"github.com/ProtonMail/gluon/connector"
	"github.com/ProtonMail/gluon/imap"

	_ "github.com/bradenaw/juniper/xslices"
	"github.com/enjoys-in/airsend-imap/internal/core/queries"
	user_interfaces "github.com/enjoys-in/airsend-imap/internal/interfaces/user"
	"github.com/enjoys-in/airsend-imap/internal/utils/encryption"
	"github.com/enjoys-in/airsend-imap/internal/utils/ticker"
)

// MyDatabaseConnector implements the connector.Connector interface
var _ connector.Connector = (*MyDatabaseConnector)(nil)
var (
	ErrNoSuchMailbox = errors.New("no such mailbox")
	ErrNoSuchMessage = errors.New("no such message")

	ErrInvalidPrefix   = errors.New("invalid prefix")
	ErrRenameForbidden = errors.New("rename operation is not allowed")
	ErrDeleteForbidden = errors.New("delete operation is not allowed")
)

type MyDatabaseConnector struct {
	DB                         *sql.DB
	Email                      string
	state                      *MailboxState
	User                       *user_interfaces.UserConfig
	Updates                    chan imap.Update
	LastClientIMAPID           imap.IMAPID
	AllowUnknownMailbox        bool
	FolderPrefix, LabelsPrefix string
	UpdatesAllowedToFail       int32

	queueLock           sync.Mutex
	queue               []imap.Update
	MailboxVisibilities map[imap.MailboxID]imap.MailboxVisibility
	ticker              *ticker.Ticker
}

// Constructor
func NewMyDatabaseConnector(db *sql.DB, email string) *MyDatabaseConnector {
	connector := &MyDatabaseConnector{
		DB:    db,
		Email: email,
		state: newMailboxState(imap.NewFlagSet(
			imap.FlagSeen,
			imap.FlagAnswered,
			imap.FlagFlagged,
			imap.FlagDeleted,
			imap.FlagDraft,
			"$Important",
			"$Pinned",
			"$Archived",
		), imap.NewFlagSet(
			imap.FlagSeen,
			imap.FlagAnswered,
			imap.FlagFlagged,
			imap.FlagDeleted,
			imap.FlagDraft,
			"$Important",
			"$Pinned",
			"$Archived",
		), imap.NewFlagSet()),
		Updates:             make(chan imap.Update, 1000),
		LastClientIMAPID:    imap.IMAPID{},
		AllowUnknownMailbox: false,
		FolderPrefix:        "",
		LabelsPrefix:        "",
		User:                nil, // or initialize properly if needed
		MailboxVisibilities: make(map[imap.MailboxID]imap.MailboxVisibility),

		ticker: ticker.New(time.Second),
	}
	go func() {
		connector.ticker.Tick(func(time.Time) {
			for _, update := range connector.popUpdates() {
				defer func() {
					err, ok := update.Wait()
					if ok && err != nil {
						if atomic.LoadInt32(&connector.UpdatesAllowedToFail) == 0 {
							panic(fmt.Sprintf("Failed to apply update %v: %v", update.String(), err))
						} else {
							fmt.Errorf("Failed to apply update %v: %v", update.String(), err)
						}
					}
				}()

				connector.Updates <- update
			}
		})
	}()

	return connector
}
func (c *MyDatabaseConnector) Init(ctx context.Context, cache connector.IMAPState) error {

	// return cache.Write(ctx, func(ctx context.Context, iw connector.IMAPStateWrite) error {
	// 	mboxes, err := iw.GetMailboxesWithoutAttrib(ctx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	for _, mbox := range mboxes {
	// 		if err := iw.PatchMailboxHierarchyWithoutTransforms(ctx, mbox.ID, xslices.Insert(mbox.Name, 0, "/fd")); err != nil {
	// 			return fmt.Errorf("failed to update mailbox name: %w", err)
	// 		}

	// 	}

	// 	err = iw.CreateMailbox(ctx, imap.Mailbox{Name: []string{"INBOX"}, ID: imap.MailboxID("INBOX")})
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create INBOX:%w", err)
	// 	}
	// 	// Retroactively apply the forwarded flags to existing mailboxes so that the IMAP clients can recognize
	// 	// that they can store these flags now.
	// 	if err := iw.AddFlagsToAllMailboxes(ctx, imap.ForwardFlagList...); err != nil {
	// 		return fmt.Errorf("failed to add \\Forward flag to all mailboxes:%w", err)
	// 	}

	// 	// Add forwarded flag as perm flags to all mailboxes.
	// 	if err := iw.AddPermFlagsToAllMailboxes(ctx, imap.ForwardFlagList...); err != nil {
	// 		return fmt.Errorf("failed to add \\Forward permanent flag to all mailboxes:%w", err)
	// 	}
	// 	return nil
	// })
	return nil

}

// Authorize authenticates user
func (c *MyDatabaseConnector) Authorize(ctx context.Context, username string, password []byte) bool {
	var (
		id, hash, tenant, key     string
		mailboxSize, usage        int
		openPGPJSON, sysEmailJSON []byte
	)

	parts := strings.Split(username, "@")
	row := c.DB.QueryRowContext(ctx, queries.GetAuthUserQuery(), username, parts[1])
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

	isMatch := encryption.ValidatePassword(hash, string(password))

	if !isMatch {
		log.Fatal("Password Mismatch", err)
		return false
	}
	// Unmarshal JSON columns
	var openPGP user_interfaces.OpenPGPKeys
	if len(openPGPJSON) > 0 {
		if err := json.Unmarshal(openPGPJSON, &openPGP); err != nil {
			log.Printf("⚠️ Failed to parse OpenPGP JSON for %s: %v", username, err)
		}
	}

	var sysEmail user_interfaces.SystemEmail
	if len(sysEmailJSON) > 0 {
		if err := json.Unmarshal(sysEmailJSON, &sysEmail); err != nil {
			log.Printf("⚠️ Failed to parse SystemEmail JSON for %s: %v", username, err)
		}
	}

	user := &user_interfaces.UserConfig{
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
	c.User = user

	return true
}

// CreateMailbox creates mailbox
func (c *MyDatabaseConnector) CreateMailbox(ctx context.Context, cache connector.IMAPStateWrite, name []string) (imap.Mailbox, error) {
	fmt.Print("103")
	mboxName := name[len(name)-1]
	var mboxID string
	err := c.DB.QueryRowContext(ctx,
		"INSERT INTO mailboxes (user_id, name, created_at) VALUES ($1,$2,$3) RETURNING id",
		c.User.Email, mboxName, time.Now(),
	).Scan(&mboxID)
	if err != nil {
		return imap.Mailbox{}, err
	}

	mbox := imap.Mailbox{
		ID:   imap.MailboxID(mboxID),
		Name: name,
	}
	c.Updates <- imap.NewMailboxCreated(mbox)
	return mbox, nil
}

// GetMessageLiteral fetches raw content
func (c *MyDatabaseConnector) GetMessageLiteral(ctx context.Context, id imap.MessageID) ([]byte, error) {
	fmt.Print("124")

	var literal []byte
	err := c.DB.QueryRowContext(ctx, "SELECT raw_content FROM messages WHERE id=$1 AND user_id=$2",
		string(id), c.User.Email).Scan(&literal)
	return literal, err
}

// GetMailboxVisibility returns visibility
func (c *MyDatabaseConnector) GetMailboxVisibility(ctx context.Context, mboxID imap.MailboxID) imap.MailboxVisibility {
	fmt.Print("134")

	return imap.Visible
}

// UpdateMailboxName updates mailbox
func (c *MyDatabaseConnector) UpdateMailboxName(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID, newName []string) error {
	_, err := c.DB.ExecContext(ctx, "UPDATE mailboxes SET name=$1 WHERE id=$2 AND user_id=$3",
		newName[len(newName)-1], string(mboxID), c.User.Email)
	return err
}

// DeleteMailbox deletes mailbox
func (c *MyDatabaseConnector) DeleteMailbox(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID) error {
	_, err := c.DB.ExecContext(ctx, "DELETE FROM mailboxes WHERE id=$1 AND user_id=$2",
		string(mboxID), c.User.Email)
	return err
}

// CreateMessage inserts message
func (c *MyDatabaseConnector) CreateMessage(ctx context.Context, cache connector.IMAPStateWrite, mboxID imap.MailboxID, literal []byte, flags imap.FlagSet, date time.Time) (imap.Message, []byte, error) {
	var msgID string
	err := c.DB.QueryRowContext(ctx,
		"INSERT INTO messages (user_id, mailbox_id, raw_content, seen, flagged, date, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id",
		c.User.Email, string(mboxID), literal, flags.Contains(imap.FlagSeen), flags.Contains(imap.FlagFlagged), date, time.Now(),
	).Scan(&msgID)
	if err != nil {
		return imap.Message{}, nil, err
	}
	msg := imap.Message{ID: imap.MessageID(msgID), Flags: flags, Date: date}
	return msg, literal, nil
}

// AddMessagesToMailbox associates messages
func (c *MyDatabaseConnector) AddMessagesToMailbox(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	for _, msgID := range messageIDs {
		_, err := c.DB.ExecContext(ctx, "INSERT INTO message_mailbox (message_id, mailbox_id) VALUES ($1,$2)",
			string(msgID), string(mboxID))
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveMessagesFromMailbox dissociates messages
func (c *MyDatabaseConnector) RemoveMessagesFromMailbox(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxID imap.MailboxID) error {
	for _, msgID := range messageIDs {
		_, err := c.DB.ExecContext(ctx, "DELETE FROM message_mailbox WHERE message_id=$1 AND mailbox_id=$2",
			string(msgID), string(mboxID))
		if err != nil {
			return err
		}
	}
	return nil
}

// MoveMessages moves messages atomically
func (c *MyDatabaseConnector) MoveMessages(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, mboxFromID, mboxToID imap.MailboxID) (bool, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, msgID := range messageIDs {
		_, err := tx.ExecContext(ctx,
			"UPDATE messages SET mailbox_id=$1 WHERE id=$2 AND mailbox_id=$3 AND user_id=$4",
			string(mboxToID), string(msgID), string(mboxFromID), c.User.Email)
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
func (c *MyDatabaseConnector) MarkMessagesSeen(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, seen bool) error {
	for _, msgID := range messageIDs {
		_, err := c.DB.ExecContext(ctx, "UPDATE messages SET seen=$1 WHERE id=$2 AND user_id=$3",
			seen, string(msgID), c.User.Email)
		if err != nil {
			return err
		}
	}
	return nil
}

// MarkMessagesFlagged flags messages
func (c *MyDatabaseConnector) MarkMessagesFlagged(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, flagged bool) error {
	for _, msgID := range messageIDs {
		_, err := c.DB.ExecContext(ctx, "UPDATE messages SET flagged=$1 WHERE id=$2 AND user_id=$3",
			flagged, string(msgID), c.User.Email)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *MyDatabaseConnector) MarkMessagesForwarded(ctx context.Context, cache connector.IMAPStateWrite, messageIDs []imap.MessageID, forwarded bool) error {
	// Not implemented
	return nil
}

// GetUpdates returns channel
func (c *MyDatabaseConnector) GetUpdates() <-chan imap.Update {
	return c.Updates
}

// Close closes connector
func (c *MyDatabaseConnector) Close(ctx context.Context) error {
	close(c.Updates)
	c.ticker.Stop()
	c.User = nil
	return nil
}

// GetMailboxMessages returns all messages in a mailbox
// Triggered by: SELECT or EXAMINE commands

// ListMailboxes lists all mailboxes for the authenticated user
// func (c *MyDatabaseConnector) ListMailboxes(ctx context.Context) ([]imap.Mailbox, error) {
// 	fmt.Print("245")
// 	rows, err := c.DB.QueryContext(ctx, queries.GetMailboxOfUserQuery(), c.User.Email)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var mailboxes []imap.Mailbox
// 	for rows.Next() {
// 		var id, name string
// 		if err := rows.Scan(&id, &name); err != nil {
// 			return nil, err
// 		}
// 		mbox := imap.Mailbox{
// 			ID:   imap.MailboxID(id),
// 			Name: []string{name},
// 		}
// 		mailboxes = append(mailboxes, mbox)
// 	}
// 	return mailboxes, nil

// }

// // ListMessages lists messages in a mailbox, optionally filtered by sequence numbers
// func (c *MyDatabaseConnector) ListMessages(ctx context.Context, mboxID imap.MailboxID, seqset *imap.SeqSet) ([]imap.Message, error) {
// 	fmt.Print("265")

// 	query := "SELECT id, seen, flagged, date FROM messages WHERE mailbox_id=$1 AND user_id=$2 ORDER BY date ASC"
// 	rows, err := c.DB.QueryContext(ctx, query, string(mboxID), c.User.Email)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var messages []imap.Message
// 	seqNum := uint32(1) // IMAP sequence numbers start at 1

// 	for rows.Next() {
// 		var id string
// 		var seen, flagged bool
// 		var date time.Time
// 		if err := rows.Scan(&id, &seen, &flagged, &date); err != nil {
// 			return nil, err
// 		}

// 		flags := imap.FlagSet{}
// 		if seen {
// 			flags.Add(imap.FlagSeen)
// 		}
// 		if flagged {
// 			flags.Add(imap.FlagFlagged)
// 		}

// 		// If seqset is provided, only include messages in it
// 		if seqset != nil {
// 			seqNum++
// 			continue
// 		}

// 		messages = append(messages, imap.Message{
// 			ID:    imap.MessageID(id),
// 			Flags: flags,
// 			Date:  date,
// 		})
// 		seqNum++
// 	}

// 	return messages, nil
// }

// // GetMessageFlags returns flags for a set of message IDs
// func (c *MyDatabaseConnector) GetMessageFlags(ctx context.Context, messageIDs []imap.MessageID) (map[imap.MessageID]imap.FlagSet, error) {
// 	flagsMap := make(map[imap.MessageID]imap.FlagSet)

// 	for _, msgID := range messageIDs {
// 		var seen, flagged bool
// 		err := c.DB.QueryRowContext(ctx, "SELECT seen, flagged FROM messages WHERE id=$1 AND user_id=$2",
// 			string(msgID), c.User.Email).Scan(&seen, &flagged)
// 		if err != nil {
// 			return nil, err
// 		}

// 		flags := imap.FlagSet{}
// 		if seen {
// 			flags.Add(imap.FlagSeen)
// 		}
// 		if flagged {
// 			flags.Add(imap.FlagFlagged)
// 		}

// 		flagsMap[msgID] = flags
// 	}

// 	return flagsMap, nil
// }

// func (c *MyDatabaseConnector) GetMailboxMessages(ctx context.Context, mboxID imap.MailboxID) ([]imap.Message, error) {
// 	log.Printf("GetMailboxMessages: Fetching messages for mailbox %s", mboxID)
// 	fmt.Print("351")

// 	rows, err := c.DB.QueryContext(ctx,
// 		queries.GetMailboxByIDQuery(),
// 		string(mboxID), c.User.Email,
// 	)
// 	if err != nil {
// 		log.Printf("GetMailboxMessages: Query failed: %v", err)
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	messages := []imap.Message{}
// 	for rows.Next() {
// 		var msgID string
// 		var seen, flagged, deleted bool
// 		var date time.Time

// 		if err := rows.Scan(&msgID, &seen, &flagged, &deleted, &date); err != nil {
// 			log.Printf("GetMailboxMessages: Scan failed: %v", err)
// 			continue
// 		}

// 		// Build message flags
// 		flags := imap.NewFlagSet()
// 		if seen {
// 			flags = flags.Add(imap.FlagSeen)
// 		}
// 		if flagged {
// 			flags = flags.Add(imap.FlagFlagged)
// 		}
// 		if deleted {
// 			flags = flags.Add(imap.FlagDeleted)
// 		}

// 		messages = append(messages, imap.Message{
// 			ID:    imap.MessageID(msgID),
// 			Flags: flags,
// 			Date:  date,
// 		})
// 	}

// 	log.Printf("GetMailboxMessages: Returned %d messages for mailbox %s", len(messages), mboxID)
// 	return messages, nil
// }

// // GetMailboxMessageCount returns the count of messages in a mailbox
// // Triggered by: STATUS command
// func (c *MyDatabaseConnector) GetMailboxMessageCount(ctx context.Context, mboxID imap.MailboxID) (int, error) {
// 	log.Printf("GetMailboxMessageCount: Counting messages for mailbox %s", mboxID)

// 	var count int
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT COUNT(*) FROM messages WHERE mailbox_id = $1 AND user_id = $2",
// 		string(mboxID), c.User.Email,
// 	).Scan(&count)

// 	if err != nil {
// 		log.Printf("GetMailboxMessageCount: Query failed: %v", err)
// 		return 0, err
// 	}

// 	log.Printf("GetMailboxMessageCount: Mailbox %s has %d messages", mboxID, count)
// 	return count, nil
// }

// // Expunge permanently removes messages marked with \Deleted flag
// // Triggered by: EXPUNGE command
// func (c *MyDatabaseConnector) Expunge(ctx context.Context, mboxID imap.MailboxID) error {
// 	log.Printf("Expunge: Removing deleted messages from mailbox %s", mboxID)

// 	// First, get the IDs of messages to be deleted (for logging/audit)
// 	rows, err := c.DB.QueryContext(ctx,
// 		"SELECT id FROM messages WHERE mailbox_id = $1 AND user_id = $2 AND deleted = true",
// 		string(mboxID), c.User.Email,
// 	)
// 	if err != nil {
// 		log.Printf("Expunge: Failed to query deleted messages: %v", err)
// 		return err
// 	}

// 	messageIDs := []string{}
// 	for rows.Next() {
// 		var msgID string
// 		if err := rows.Scan(&msgID); err == nil {
// 			messageIDs = append(messageIDs, msgID)
// 		}
// 	}
// 	rows.Close()

// 	if len(messageIDs) == 0 {
// 		log.Printf("Expunge: No messages to expunge in mailbox %s", mboxID)
// 		return nil
// 	}

// 	// Delete the messages
// 	result, err := c.DB.ExecContext(ctx,
// 		"DELETE FROM messages WHERE mailbox_id = $1 AND user_id = $2 AND deleted = true",
// 		string(mboxID), c.User.Email,
// 	)
// 	if err != nil {
// 		log.Printf("Expunge: Failed to delete messages: %v", err)
// 		return err
// 	}

// 	rowsAffected, _ := result.RowsAffected()
// 	log.Printf("Expunge: Successfully removed %d messages from mailbox %s", rowsAffected, mboxID)

// 	// Optional: Log the deleted message IDs for audit trail
// 	for _, msgID := range messageIDs {
// 		log.Printf("Expunge: Deleted message %s", msgID)
// 	}

// 	// Optional: Notify Gluon about deleted messages
// 	for _, msgID := range messageIDs {
// 		c.Updates <- &imap.MessageDeleted{
// 			MessageID: imap.MessageID(msgID),
// 		}
// 	}

// 	return nil
// }

// // GetUnseenCount returns count of unseen messages (useful for STATUS)
// func (c *MyDatabaseConnector) GetUnseenCount(ctx context.Context, mboxID imap.MailboxID) (int, error) {
// 	var count int
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT COUNT(*) FROM messages WHERE mailbox_id = $1 AND user_id = $2 AND seen = false",
// 		string(mboxID), c.User.Email,
// 	).Scan(&count)

// 	return count, err
// }

// // GetRecentCount returns count of recent messages (useful for STATUS)
// func (c *MyDatabaseConnector) GetRecentCount(ctx context.Context, mboxID imap.MailboxID) (int, error) {
// 	var count int
// 	err := c.DB.QueryRowContext(ctx,
// 		`SELECT COUNT(*) FROM messages
// 		WHERE mailbox_id = $1 AND user_id = $2
// 		AND created_at > NOW() - INTERVAL '24 hours'`,
// 		string(mboxID), c.User.Email,
// 	).Scan(&count)

// 	return count, err
// }

// // ExpungeAll expunges deleted messages from all mailboxes (useful for maintenance)
// func (c *MyDatabaseConnector) ExpungeAll(ctx context.Context) error {
// 	log.Printf("ExpungeAll: Removing all deleted messages for user %s", c.User.Email)

// 	result, err := c.DB.ExecContext(ctx,
// 		"DELETE FROM messages WHERE user_id = $1 AND deleted = true",
// 		c.User.Email,
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	rowsAffected, _ := result.RowsAffected()
// 	log.Printf("ExpungeAll: Removed %d total messages", rowsAffected)

// 	return nil
// }

// // ============================================================
// // 1. LABELS SUPPORT INTERFACE
// // Gmail-style labels (multiple mailboxes per message)
// // ============================================================

// // AddLabelsToMessages adds labels (mailboxes) to messages
// // Triggered by: Gmail X-GM-LABELS extension
// func (c *MyDatabaseConnector) AddLabelsToMessages(ctx context.Context, messageIDs []imap.MessageID, labelIDs []imap.MailboxID) error {
// 	log.Printf("AddLabelsToMessages: Adding %d labels to %d messages", len(labelIDs), len(messageIDs))

// 	for _, msgID := range messageIDs {
// 		for _, labelID := range labelIDs {
// 			_, err := c.DB.ExecContext(ctx,
// 				`INSERT INTO message_labels (message_id, label_id, user_id)
// 				VALUES ($1, $2, $3)
// 				ON CONFLICT (message_id, label_id) DO NOTHING`,
// 				string(msgID), string(labelID), c.User.Email,
// 			)
// 			if err != nil {
// 				log.Printf("AddLabelsToMessages: Failed to add label %s to message %s: %v", labelID, msgID, err)
// 				return err
// 			}
// 		}
// 	}

// 	log.Printf("AddLabelsToMessages: Successfully added labels")
// 	return nil
// }

// // RemoveLabelsFromMessages removes labels from messages
// // Triggered by: Gmail X-GM-LABELS extension
// func (c *MyDatabaseConnector) RemoveLabelsFromMessages(ctx context.Context, messageIDs []imap.MessageID, labelIDs []imap.MailboxID) error {
// 	log.Printf("RemoveLabelsFromMessages: Removing %d labels from %d messages", len(labelIDs), len(messageIDs))

// 	for _, msgID := range messageIDs {
// 		for _, labelID := range labelIDs {
// 			_, err := c.DB.ExecContext(ctx,
// 				"DELETE FROM message_labels WHERE message_id = $1 AND label_id = $2 AND user_id = $3",
// 				string(msgID), string(labelID), c.User.Email,
// 			)
// 			if err != nil {
// 				log.Printf("RemoveLabelsFromMessages: Failed to remove label %s from message %s: %v", labelID, msgID, err)
// 				return err
// 			}
// 		}
// 	}

// 	log.Printf("RemoveLabelsFromMessages: Successfully removed labels")
// 	return nil
// }

// // GetMessageLabels returns all labels for a message
// // Triggered by: Gmail X-GM-LABELS extension
// func (c *MyDatabaseConnector) GetMessageLabels(ctx context.Context, messageID imap.MessageID) ([]imap.MailboxID, error) {
// 	log.Printf("GetMessageLabels: Getting labels for message %s", messageID)

// 	rows, err := c.DB.QueryContext(ctx,
// 		"SELECT label_id FROM message_labels WHERE message_id = $1 AND user_id = $2",
// 		string(messageID), c.User.Email,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	labels := []imap.MailboxID{}
// 	for rows.Next() {
// 		var labelID string
// 		if err := rows.Scan(&labelID); err != nil {
// 			continue
// 		}
// 		labels = append(labels, imap.MailboxID(labelID))
// 	}

// 	log.Printf("GetMessageLabels: Message %s has %d labels", messageID, len(labels))
// 	return labels, nil
// }

// // ============================================================
// // 2. UID VALIDITY BUMP INTERFACE
// // For handling UID validity changes
// // ============================================================

// // BumpUIDValidity increments the UID validity for a mailbox
// // Called when mailbox structure changes dramatically (like rebuild)
// // Triggered by: Manual operations or mailbox repairs
// func (c *MyDatabaseConnector) BumpUIDValidity(ctx context.Context, mboxID imap.MailboxID) error {
// 	log.Printf("BumpUIDValidity: Bumping UID validity for mailbox %s", mboxID)

// 	// Update UID validity in database
// 	_, err := c.DB.ExecContext(ctx,
// 		"UPDATE public.mail_folders SET uid_validity = uid_validity + 1 WHERE id = $1 AND user_id = $2",
// 		string(mboxID), c.User.Email,
// 	)
// 	if err != nil {
// 		log.Printf("BumpUIDValidity: Failed to bump UID validity: %v", err)
// 		return err
// 	}

// 	// Notify Gluon
// 	//uid := uint32(rand.Intn(999999-100000+1) + 100000)
// 	c.Updates <- imap.NewUIDValidityBumped()

// 	log.Printf("BumpUIDValidity: Successfully bumped UID validity for mailbox %s", mboxID)
// 	return nil
// }

// // ============================================================
// // 3. IMAP ID INTERFACE
// // For IMAP ID extension support
// // ============================================================

// // IMAPID returns server identification information
// // Triggered by: IMAP ID command
// func (c *MyDatabaseConnector) IMAPID(ctx context.Context) imap.IMAPID {
// 	log.Printf("IMAPID: Client requested server ID")

// 	return imap.IMAPID{
// 		Name:       "ENJOYS IMAP Server",
// 		Version:    "1.0.0",
// 		Vendor:     "ENJOYS",
// 		SupportURL: "su@enjoys.in",
// 		Date:       time.Now().Format("2006-01-02"),
// 	}
// }

// // ============================================================
// // 4. QUOTA SUPPORT INTERFACE
// // For IMAP QUOTA extension
// // ============================================================

// // GetQuota returns quota information for a quota root
// // Triggered by: GETQUOTA command
// // func (c *MyDatabaseConnector) GetQuota(ctx context.Context, quotaRoot string) (imap.Quota, error) {
// // 	log.Printf("GetQuota: Getting quota for root '%s'", quotaRoot)

// // 	var used, limit int64
// // 	err := c.DB.QueryRowContext(ctx,
// // 		"SELECT used_bytes, limit_bytes FROM user_quotas WHERE user_id = $1",
// // 		c.User.Email,
// // 	).Scan(&used, &limit)

// // 	if err != nil {
// // 		if err == sql.ErrNoRows {
// // 			// No quota set, return unlimited
// // 			return imap.Quota{
// // 				Name:  quotaRoot,
// // 				Used:  0,
// // 				Limit: 0, // 0 means unlimited
// // 			}, nil
// // 		}
// // 		return imap.Quota{}, err
// // 	}

// // 	quota := imap.Quota{
// // 		Name:  quotaRoot,
// // 		Used:  used,
// // 		Limit: limit,
// // 	}

// // 	log.Printf("GetQuota: User %s - Used: %d bytes, Limit: %d bytes", c.User.Email, used, limit)
// // 	return quota, nil
// // }

// // GetQuotaRoot returns the quota root for a mailbox
// // Triggered by: GETQUOTAROOT command
// func (c *MyDatabaseConnector) GetQuotaRoot(ctx context.Context, mboxID imap.MailboxID) (string, error) {
// 	log.Printf("GetQuotaRoot: Getting quota root for mailbox %s", mboxID)

// 	// In most cases, quota root is the username or user ID
// 	quotaRoot := fmt.Sprintf("user/%s", c.User.Email)

// 	log.Printf("GetQuotaRoot: Quota root for mailbox %s is '%s'", mboxID, quotaRoot)
// 	return quotaRoot, nil
// }

// // ============================================================
// // 5. METADATA SUPPORT INTERFACE
// // For IMAP METADATA extension
// // ============================================================

// // GetMetadata retrieves metadata for a mailbox
// // Triggered by: GETMETADATA command
// func (c *MyDatabaseConnector) GetMetadata(ctx context.Context, mboxID imap.MailboxID, keys []string) (map[string]string, error) {
// 	log.Printf("GetMetadata: Getting metadata for mailbox %s, keys: %v", mboxID, keys)

// 	metadata := make(map[string]string)

// 	for _, key := range keys {
// 		var value string
// 		err := c.DB.QueryRowContext(ctx,
// 			"SELECT value FROM mailbox_metadata WHERE mailbox_id = $1 AND key = $2 AND user_id = $3",
// 			string(mboxID), key, c.User.Email,
// 		).Scan(&value)

// 		if err == nil {
// 			metadata[key] = value
// 		} else if err != sql.ErrNoRows {
// 			log.Printf("GetMetadata: Error getting key %s: %v", key, err)
// 		}
// 	}

// 	log.Printf("GetMetadata: Retrieved %d metadata entries", len(metadata))
// 	return metadata, nil
// }

// // SetMetadata sets metadata for a mailbox
// // Triggered by: SETMETADATA command
// func (c *MyDatabaseConnector) SetMetadata(ctx context.Context, mboxID imap.MailboxID, metadata map[string]string) error {
// 	log.Printf("SetMetadata: Setting %d metadata entries for mailbox %s", len(metadata), mboxID)

// 	for key, value := range metadata {
// 		_, err := c.DB.ExecContext(ctx,
// 			`INSERT INTO mailbox_metadata (mailbox_id, key, value, user_id, updated_at)
// 			VALUES ($1, $2, $3, $4, $5)
// 			ON CONFLICT (mailbox_id, key)
// 			DO UPDATE SET value = $3, updated_at = $5`,
// 			string(mboxID), key, value, c.User.Email, time.Now(),
// 		)
// 		if err != nil {
// 			log.Printf("SetMetadata: Failed to set key %s: %v", key, err)
// 			return err
// 		}
// 	}

// 	log.Printf("SetMetadata: Successfully set metadata")
// 	return nil
// }

// // ============================================================
// // 6. SPECIAL USE INTERFACE
// // For marking special mailboxes (Drafts, Sent, Trash, etc.)
// // ============================================================

// // GetMailboxAttributes returns special-use attributes for a mailbox
// // Triggered by: LIST command with SPECIAL-USE extension
// func (c *MyDatabaseConnector) GetMailboxAttributes(ctx context.Context, mboxID imap.MailboxID) ([]string, error) {
// 	log.Printf("GetMailboxAttributes: Getting attributes for mailbox %s", mboxID)

// 	var attributes string
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT attributes FROM mailboxes WHERE id = $1 AND user_id = $2",
// 		string(mboxID), c.User.Email,
// 	).Scan(&attributes)

// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return []string{}, nil
// 		}
// 		return nil, err
// 	}

// 	// Parse attributes from database (stored as comma-separated or JSON)
// 	// Common attributes: \Drafts, \Sent, \Trash, \Junk, \Flagged, \All, \Archive
// 	attrList := []string{}
// 	if attributes != "" {
// 		// Simple comma-separated parsing
// 		// In production, use proper JSON or array handling
// 		attrList = append(attrList, attributes)
// 	}

// 	log.Printf("GetMailboxAttributes: Mailbox %s has attributes: %v", mboxID, attrList)
// 	return attrList, nil
// }

// // ============================================================
// // 7. IDLE INTERFACE
// // For IMAP IDLE command support (push notifications)
// // ============================================================

// // Idle implements long-running notification mechanism
// // Triggered by: IDLE command
// func (c *MyDatabaseConnector) Idle(ctx context.Context, mboxID imap.MailboxID, updateCh chan<- imap.Update) error {
// 	log.Printf("Idle: Starting IDLE for mailbox %s", mboxID)

// 	// Create a ticker to poll for changes (in production, use database triggers or pub/sub)
// 	ticker := time.NewTicker(30 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Printf("Idle: IDLE cancelled for mailbox %s", mboxID)
// 			return ctx.Err()

// 		case <-ticker.C:
// 			// Check for new messages
// 			var count int
// 			err := c.DB.QueryRowContext(ctx,
// 				`SELECT COUNT(*) FROM messages
// 				WHERE mailbox_id = $1 AND user_id = $2
// 				AND created_at > NOW() - INTERVAL '1 minute'`,
// 				string(mboxID), c.User.Email,
// 			).Scan(&count)

// 			if err != nil {
// 				log.Printf("Idle: Error checking for updates: %v", err)
// 				continue
// 			}

// 			if count > 0 {
// 				log.Printf("Idle: Detected %d new messages in mailbox %s", count, mboxID)
// 				// Send EXISTS update to client
// 				updateCh <- &imap.MailboxUpdated{
// 					MailboxID: mboxID,
// 				}
// 			}
// 		}
// 	}
// }

// // ============================================================
// // 8. APPEND LIMIT INTERFACE
// // For controlling max message size
// // ============================================================

// // GetAppendLimit returns the maximum size for APPEND command
// // Triggered by: Client capability negotiation
// func (c *MyDatabaseConnector) GetAppendLimit(ctx context.Context) int64 {
// 	log.Printf("GetAppendLimit: Client requested append limit")

// 	// Return max message size (e.g., 50MB)
// 	const maxMessageSize = 50 * 1024 * 1024 // 50 MB

// 	log.Printf("GetAppendLimit: Returning limit of %d bytes", maxMessageSize)
// 	return maxMessageSize
// }

// // ============================================================
// // 10. ADVANCED CONNECTOR METHODS
// // ============================================================

// // GetMessageSize returns message size without fetching full content
// // Triggered by: FETCH RFC822.SIZE
// func (c *MyDatabaseConnector) GetMessageSize(ctx context.Context, messageID imap.MessageID) (int64, error) {
// 	log.Printf("GetMessageSize: Getting size for message %s", messageID)

// 	var size int64
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT LENGTH(raw_content) FROM messages WHERE id = $1 AND user_id = $2",
// 		string(messageID), c.User.Email,
// 	).Scan(&size)

// 	if err != nil {
// 		return 0, err
// 	}

// 	log.Printf("GetMessageSize: Message %s is %d bytes", messageID, size)
// 	return size, nil
// }

// // GetMessageStructure returns MIME structure of a message
// // Triggered by: FETCH BODYSTRUCTURE
// func (c *MyDatabaseConnector) GetMessageStructure(ctx context.Context, messageID imap.MessageID) (interface{}, error) {
// 	log.Printf("GetMessageStructure: Getting structure for message %s", messageID)

// 	// In production, parse MIME structure from raw_content
// 	// For now, return a simplified structure
// 	var structure string
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT mime_structure FROM messages WHERE id = $1 AND user_id = $2",
// 		string(messageID), c.User.Email,
// 	).Scan(&structure)

// 	if err != nil {
// 		// If mime_structure not cached, would need to parse raw_content
// 		return nil, err
// 	}

// 	log.Printf("GetMessageStructure: Retrieved structure for message %s", messageID)
// 	return structure, nil
// }

// // GetMessageHeaders returns only message headers
// // Triggered by: FETCH BODY[HEADER] or FETCH BODY.PEEK[HEADER]
// func (c *MyDatabaseConnector) GetMessageHeaders(ctx context.Context, messageID imap.MessageID) ([]byte, error) {
// 	log.Printf("GetMessageHeaders: Getting headers for message %s", messageID)

// 	var headers []byte
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT headers FROM messages WHERE id = $1 AND user_id = $2",
// 		string(messageID), c.User.Email,
// 	).Scan(&headers)

// 	if err != nil {
// 		// If headers not cached separately, extract from raw_content
// 		var rawContent []byte
// 		err := c.DB.QueryRowContext(ctx,
// 			"SELECT raw_content FROM messages WHERE id = $1 AND user_id = $2",
// 			string(messageID), c.User.Email,
// 		).Scan(&rawContent)

// 		if err != nil {
// 			return nil, err
// 		}

// 		// Extract headers (up to first \r\n\r\n or \n\n)
// 		// In production, use proper email parser
// 		headers = extractHeaders(rawContent)
// 	}

// 	log.Printf("GetMessageHeaders: Retrieved %d bytes of headers", len(headers))
// 	return headers, nil
// }

// // GetMessageBodySection returns specific MIME part
// // Triggered by: FETCH BODY[1], FETCH BODY[TEXT], etc.
// func (c *MyDatabaseConnector) GetMessageBodySection(ctx context.Context, messageID imap.MessageID, section string) ([]byte, error) {
// 	log.Printf("GetMessageBodySection: Getting section '%s' for message %s", section, messageID)

// 	// In production, parse MIME structure and extract requested section
// 	// For now, simplified implementation
// 	var rawContent []byte
// 	err := c.DB.QueryRowContext(ctx,
// 		"SELECT raw_content FROM messages WHERE id = $1 AND user_id = $2",
// 		string(messageID), c.User.Email,
// 	).Scan(&rawContent)

// 	if err != nil {
// 		return nil, err
// 	}

// 	// Extract requested section based on MIME structure
// 	// section examples: "1", "1.1", "TEXT", "HEADER.FIELDS (FROM TO)"
// 	bodySection := extractMIMESection(rawContent, section)

// 	log.Printf("GetMessageBodySection: Retrieved %d bytes for section '%s'", len(bodySection), section)
// 	return bodySection, nil
// }

// func defaultMailboxFlags() imap.FlagSet {
// 	f := imap.NewFlagSet(imap.FlagSeen, imap.FlagFlagged, imap.FlagDeleted)
// 	f.AddToSelf()

// 	return f
// }

// func defaultMailboxPermanentFlags() imap.FlagSet {
// 	return defaultMailboxFlags()
// }

// func defaultMailboxAttributes() imap.FlagSet {

// 	return imap.NewFlagSet()
// }

//https://github.com/ProtonMail/proton-bridge/blob/master/internal/services/imapservice/helpers.go
