package imap

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/ProtonMail/gluon/imap"
	"github.com/enjoys-in/airsend-imap/internal/core/queries"
)

type MessagePriority string

const (
	PriorityHigh   MessagePriority = "high"
	PriorityNormal MessagePriority = "normal"
	PriorityLow    MessagePriority = "low"
)

// syncUserData loads mailboxes and messages from DB and pushes to Gluon
func (c *MyDatabaseConnector) syncUserDataAfterAuth(ctx context.Context) error {
	// Load mailboxes
	rows, err := c.db.QueryContext(ctx,
		queries.GetMailboxOfUserQuery(),
		c.user.Email,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, path, delimiter, listed, subscribed string
		var uid_validity uint32
		if err := rows.Scan(&id, &title, &path, &delimiter, &listed, &subscribed, &uid_validity); err != nil {
			continue
		}

		c.updates <- imap.NewMailboxCreated(imap.Mailbox{
			ID:   imap.MailboxID(id),
			Name: []string{title},
			Flags: imap.NewFlagSet(
				imap.FlagSeen,
				imap.FlagAnswered,
				imap.FlagFlagged,
				imap.FlagDeleted,
				imap.FlagDraft,
				"$Important",
				"$Pinned",
				"$Archived",
			),
			PermanentFlags: imap.NewFlagSet(
				imap.FlagSeen,
				imap.FlagAnswered,
				imap.FlagFlagged,
				imap.FlagDeleted,
				imap.FlagDraft,
				"$Important",
				"$Pinned",
				"$Archived",
			),
			Attributes: getMailboxAttributes(title, false /* hasChildren */),
		})

		// Load messages for this mailbox
		c.loadMailboxMessages(ctx, imap.MailboxID(id))
	}

	return nil
}
func getMailboxAttributes(mailboxName string, hasChildren bool) imap.FlagSet {
	attrs := imap.NewFlagSet()
	if hasChildren {
		attrs.AddToSelf(imap.AttrNoInferiors)
	}
	// Add special-use attributes based on mailbox name
	switch strings.ToLower(mailboxName) {
	case "sent":
		attrs = imap.NewFlagSet(imap.AttrSent)
	case "drafts":
		attrs = imap.NewFlagSet(imap.AttrDrafts)
	case "trash", "deleted":
		attrs = imap.NewFlagSet(imap.AttrTrash)
	case "spam", "junk":
		attrs = imap.NewFlagSet(imap.AttrJunk)
	case "archive":
		attrs = imap.NewFlagSet(imap.AttrArchive)
	case "important":
		attrs = imap.NewFlagSet(imap.AttrMarked)
	case "inbox", "all":
		attrs = imap.NewFlagSet(imap.AttrNoSelect)
	}

	return attrs
}

// loadMailboxMessages loads messages for a specific mailbox
func (c *MyDatabaseConnector) loadMailboxMessages(ctx context.Context, mboxID imap.MailboxID) error {
	log.Printf("Loading messages for mailbox %s", mboxID)
	rows, err := c.db.QueryContext(ctx,
		queries.GetMailboxByIDQuery(),
		string(mboxID), // folder id
		c.user.Email,   // user email
	)
	if err != nil {
		log.Printf("Failed to query messages: %v", err)
		return err
	}
	defer rows.Close()

	messages := []*imap.MessageCreated{}

	for rows.Next() {
		var (
			messageID                                                      string
			priority                                                       MessagePriority
			isRead, isPinned, isReplied, isDeleted, isImportant, isStarred bool
			threadID                                                       sql.NullString
			tags                                                           []byte // JSON array
			plainText                                                      sql.NullString
			folder                                                         string
			content                                                        []byte // Raw email content
			timestamp                                                      time.Time
			literal                                                        []byte
		)

		err := rows.Scan(
			&messageID,
			&priority,
			&isRead,
			&isPinned,
			&isReplied,
			&threadID,
			&isDeleted,
			&isImportant,
			&isStarred,
			&tags,
			&plainText,
			&folder,
			&content,
			&timestamp,
		)

		if err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		// Build IMAP flags based on your database fields
		flags := imap.NewFlagSet()

		// Standard IMAP flags
		if isRead {
			flags = flags.Add(imap.FlagSeen)
		}
		if isStarred {
			flags = flags.Add(imap.FlagFlagged)
		}
		if isDeleted {
			flags = flags.Add(imap.FlagDeleted)
		}
		if isReplied {
			flags = flags.Add(imap.FlagAnswered)
		}

		// Gmail/Outlook specific flags (as custom keywords)
		// These appear as custom flags in IMAP clients
		if isImportant {
			flags = flags.Add("$Important")
		}
		if isPinned {
			flags = flags.Add("$Pinned")
		}

		// Handle priority flags
		switch priority {
		case PriorityHigh:
			flags = flags.Add("$HighPriority")
		case PriorityLow:
			flags = flags.Add("$LowPriority")
		default:
			flags = flags.Add("$NormalPriority")
		}

		// Parse and add custom tags as IMAP keywords
		if len(tags) > 0 {
			var tagList []string
			if err := json.Unmarshal(tags, &tagList); err == nil {
				for _, tag := range tagList {
					// Add each tag as a custom keyword
					// IMAP keywords should start with $ or be alphanumeric
					flags = flags.Add(tag)
				}
			}
		}

		// Create IMAP message
		// Decode First layer of MEssage base64 -> OpenPGP encrypted message -> Parse OpenPGP -> Extract inner MIME message
		// parsed, err := imap.NewParsedMessage(content)
		// if err != nil {
		// 	// fallback: just send literal
		// 	parsed = nil
		// }

		messages = append(messages,
			&imap.MessageCreated{
				Message: imap.Message{
					ID:    imap.MessageID(messageID),
					Flags: flags,
					Date:  timestamp,
				},
				Literal:    literal,
				MailboxIDs: []imap.MailboxID{mboxID},
				// ParsedMessage: nil, // optional: populate if you have parsed message data
			})

	}

	// Check for any row iteration errors
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return err
	}

	// Push messages to Gluon in batches (avoid overwhelming the channel)

	if len(messages) > 0 {
		// Split into batches of 100 messages
		c.updates <- imap.NewMessagesCreated(true, messages...)
	}

	return nil
}

// Sync synchronizes the connector state with your database
// Called by Gluon to refresh mailbox and message state
// Triggered by: Periodic refresh, or when Gluon needs fresh data
func (c *MyDatabaseConnector) Sync(ctx context.Context) error {
	log.Printf("SYNC: Starting sync for user %s", c.user.Email)

	// Step 1: Sync mailboxes
	if err := c.syncMailboxes(ctx); err != nil {
		log.Printf("SYNC: Failed to sync mailboxes: %v", err)
		return err
	}

	// Step 2: Sync messages for all mailboxes
	if err := c.syncAllMessages(ctx); err != nil {
		log.Printf("SYNC: Failed to sync messages: %v", err)
		return err
	}

	log.Printf("SYNC: Completed sync for user %s", c.user.Email)
	return nil
}

// syncMailboxes loads mailboxes from database and pushes to Gluon
func (c *MyDatabaseConnector) syncMailboxes(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx,
		"SELECT id, name FROM mailboxes WHERE user_id = $1 ORDER BY name",
		c.user.Email,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	mailboxCount := 0
	for rows.Next() {
		var mboxID, name string
		if err := rows.Scan(&mboxID, &name); err != nil {
			log.Printf("SYNC: Failed to scan mailbox: %v", err)
			continue
		}

		// Push mailbox to Gluon via updates channel
		c.updates <- &imap.MailboxCreated{
			Mailbox: imap.Mailbox{
				ID:   imap.MailboxID(mboxID),
				Name: []string{name},
			},
		}
		mailboxCount++
	}

	log.Printf("SYNC: Synced %d mailboxes", mailboxCount)
	return nil
}

// syncAllMessages loads all messages and pushes to Gluon
func (c *MyDatabaseConnector) syncAllMessages(ctx context.Context) error {
	// Get all mailboxes first
	rows, err := c.db.QueryContext(ctx,
		"SELECT id FROM mailboxes WHERE user_id = $1",
		c.user.Email,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	mailboxIDs := []string{}
	for rows.Next() {
		var mboxID string
		if err := rows.Scan(&mboxID); err != nil {
			continue
		}
		mailboxIDs = append(mailboxIDs, mboxID)
	}

	// Sync messages for each mailbox
	totalMessages := 0
	for _, mboxID := range mailboxIDs {
		count, err := c.syncMessagesForMailbox(ctx, imap.MailboxID(mboxID))
		if err != nil {
			log.Printf("SYNC: Failed to sync messages for mailbox %s: %v", mboxID, err)
			continue
		}
		totalMessages += count
	}

	log.Printf("SYNC: Synced %d total messages across %d mailboxes", totalMessages, len(mailboxIDs))
	return nil
}

// syncMessagesForMailbox syncs messages for a specific mailbox
func (c *MyDatabaseConnector) syncMessagesForMailbox(ctx context.Context, mboxID imap.MailboxID) (int, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT id, seen, flagged, deleted, date 
		FROM messages 
		WHERE mailbox_id = $1 AND user_id = $2 
		ORDER BY date ASC`,
		string(mboxID), c.user.Email,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	messages := []*imap.MessageCreated{}
	for rows.Next() {
		var msgID string
		var seen, flagged, deleted bool
		var date time.Time

		if err := rows.Scan(&msgID, &seen, &flagged, &deleted, &date); err != nil {
			log.Printf("SYNC: Failed to scan message: %v", err)
			continue
		}

		// Build flags
		flags := imap.NewFlagSet()
		if seen {
			flags = flags.Add(imap.FlagSeen)
		}
		if flagged {
			flags = flags.Add(imap.FlagFlagged)
		}
		if deleted {
			flags = flags.Add(imap.FlagDeleted)
		}

		messages = append(messages, &imap.MessageCreated{
			Message: imap.Message{
				ID:    imap.MessageID(msgID),
				Flags: flags,
				Date:  date,
			},
		})
	}

	// Push messages to Gluon
	if len(messages) > 0 {
		c.updates <- &imap.MessagesCreated{
			Messages: messages,
		}
	}

	return len(messages), nil
}
