package imap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func (conn *MyDatabaseConnector) popUpdates() []imap.Update {
	conn.queueLock.Lock()
	defer conn.queueLock.Unlock()

	var updates []imap.Update

	updates, conn.queue = conn.queue, []imap.Update{}

	return updates
}

// syncUserData loads mailboxes and messages from DB and pushes to Gluon
func (c *MyDatabaseConnector) syncUserDataAfterAuth(ctx context.Context) error {
	c.queueLock.Lock()
	defer c.queueLock.Unlock()

	rows, err := c.DB.QueryContext(ctx,
		queries.GetMailboxOfUserQuery(),
		c.Email,
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
		exclusive, err := c.validateName([]string{title})
		if err != nil {
			return err
		}

		mbox := c.state.createMailbox(imap.MailboxID(id), []string{title}, exclusive)
		update := imap.NewMailboxCreated(mbox)

		c.state.mailboxes[imap.MailboxID(id)] = &MailboxOptions{
			name:      []string{title},
			exclusive: exclusive,
			id:        imap.MailboxID(id),
		}

		c.Updates <- update
		err, ok := update.WaitContext(ctx)
		if ok && err != nil {
			return fmt.Errorf("failed to apply update %v:%w", update.String(), err)
		}

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
	rows, err := c.DB.QueryContext(ctx,
		queries.GetMailboxByIDQuery(),
		string(mboxID), // folder id
		c.Email,        // user email
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
		c.Updates <- imap.NewMessagesCreated(true, messages...)
	}

	return nil
}

// Sync synchronizes the connector state with your database
// Called by Gluon to refresh mailbox and message state
// Triggered by: Periodic refresh, or when Gluon needs fresh data
func (c *MyDatabaseConnector) Sync(ctx context.Context) error {
	log.Printf("SYNC: Starting sync for user %s", c.Email)
	if err := c.syncUserDataAfterAuth(ctx); err != nil {
		log.Printf("SYNC: Failed to sync mailboxes: %v", err)
		return err
	}
	log.Printf("SYNC: Completed sync for user %s", c.Email)
	return nil
}

func (c *MyDatabaseConnector) DebugPrint(label string) {
	fmt.Printf("%s:\n", label)
	fmt.Printf("  connector ptr: %p\n", c)
	fmt.Printf("  c == nil: %v\n", c == nil)

	if c != nil {
		fmt.Printf("  db: %p (nil=%v)\n", c.DB, c.DB == nil)
		fmt.Printf("  updates: %p (nil=%v)\n", c.Updates, c.Updates == nil)
		fmt.Printf("  email: %s\n", c.Email)
	}
}
func (conn *MyDatabaseConnector) validateName(name []string) (bool, error) {
	var exclusive bool

	switch {
	case len(conn.FolderPrefix)+len(conn.LabelsPrefix) == 0:
		exclusive = false

	case len(conn.FolderPrefix) > 0 && len(conn.LabelsPrefix) > 0:
		if name[0] == conn.FolderPrefix {
			exclusive = true
		} else if name[0] == conn.LabelsPrefix {
			exclusive = false
		} else {
			return false, ErrInvalidPrefix
		}

	case len(conn.FolderPrefix) > 0:
		if len(name) > 1 && name[0] == conn.FolderPrefix {
			exclusive = true
		}

	case len(conn.LabelsPrefix) > 0:
		if len(name) > 1 && name[0] == conn.LabelsPrefix {
			exclusive = false
		}
	}

	return exclusive, nil
}
