package imap

import (
	"context"
	"github.com/ProtonMail/gluon/imap"
	"log"
	"time"
)

// syncUserData loads mailboxes and messages from DB and pushes to Gluon
func (c *MyDatabaseConnector) syncUserData(ctx context.Context) error {
	// Load mailboxes
	rows, err := c.db.QueryContext(ctx,
		"SELECT id, name FROM mailboxes WHERE user_id = $1",
		c.user.Email,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mboxID, name string
		if err := rows.Scan(&mboxID, &name); err != nil {
			continue
		}

		// Push mailbox to Gluon
		c.updates <- &imap.MailboxCreated{
			Mailbox: imap.Mailbox{
				ID:   imap.MailboxID(mboxID),
				Name: []string{name},
			},
		}

		// Load messages for this mailbox
		c.loadMailboxMessages(ctx, imap.MailboxID(mboxID))
	}

	return nil
}

// loadMailboxMessages loads messages for a specific mailbox
func (c *MyDatabaseConnector) loadMailboxMessages(ctx context.Context, mboxID imap.MailboxID) error {
	rows, err := c.db.QueryContext(ctx,
		"SELECT id, seen, flagged, deleted, date FROM messages WHERE mailbox_id = $1 AND user_id = $2",
		string(mboxID), c.user.Email,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	messages := []*imap.MessageCreated{}
	for rows.Next() {
		var msgID string
		var seen, flagged, deleted bool
		var date time.Time
		var literal []byte
		var mailboxIDs []imap.MailboxID

		// Add literal and mailboxIDs to your scan
		err := rows.Scan(&msgID, &seen, &flagged, &deleted, &date, &literal, &mailboxIDs)
		if err != nil {
			// Handle error appropriately
			continue
		}

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
			Literal:    literal,
			MailboxIDs: mailboxIDs,
			// ParsedMessage: nil, // optional: populate if you have parsed message data
		})
	}

	// Push messages to Gluon
	if len(messages) > 0 {
		c.updates <- &imap.MessagesCreated{
			Messages: messages,
		}
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
