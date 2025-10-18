package connector

import (
	"context"
	"log"
	"time"

	"github.com/ProtonMail/gluon/imap"
)

// ClearUpdates clears all pending updates in the channel
// Useful for testing or resetting state
func (c *MyDBConnector) ClearUpdates() {
	log.Printf("ClearUpdates: Clearing pending updates")

	// Drain the updates channel
	for {
		select {
		case <-c.updates:
			// Discard update
		default:
			// Channel is empty
			log.Printf("ClearUpdates: All updates cleared")
			return
		}
	}
}

// Flush flushes any pending operations
// Useful for ensuring all writes are committed
func (c *MyDBConnector) Flush() {
	log.Printf("Flush: Flushing pending operations")
	// If you have any buffering or caching, flush it here
	// For database operations, might want to ensure all transactions are committed
}

// GetLastRecordedIMAPID returns the last IMAP ID received from client
// Used for tracking client information
func (c *MyDBConnector) GetLastRecordedIMAPID() imap.IMAPID {
	log.Printf("GetLastRecordedIMAPID: Retrieving last recorded IMAP ID")

	// If you're tracking client IMAP IDs, return them here
	// This stores information sent by client via IMAP ID command
	return c.lastClientIMAPID
}

// MailboxCreated simulates external mailbox creation
// Used to test mailbox creation from external source (e.g., webmail)
func (c *MyDBConnector) MailboxCreated(mbox imap.Mailbox) error {
	log.Printf("MailboxCreated: Simulating mailbox creation: %s", mbox.Name)

	// Push update to Gluon
	c.updates <- &imap.MailboxCreated{
		Mailbox: mbox,
	}

	return nil
}

// MailboxDeleted simulates external mailbox deletion
func (c *MyDBConnector) MailboxDeleted(mboxID imap.MailboxID) error {
	log.Printf("MailboxDeleted: Simulating mailbox deletion: %s", mboxID)

	// Push update to Gluon
	c.updates <- &imap.MailboxDeleted{
		MailboxID: mboxID,
	}

	return nil
}

// MessageAdded simulates adding an existing message to a mailbox
func (c *MyDBConnector) MessageAdded(messageID imap.MessageID, mboxID imap.MailboxID) error {
	log.Printf("MessageAdded: Simulating message %s added to mailbox %s", messageID, mboxID)

	// Push update to Gluon
	c.updates <- &imap.MessageMailboxesUpdated{
		MessageID:  messageID,
		MailboxIDs: []imap.MailboxID{mboxID},
	}

	return nil
}

// MessageCreated simulates external message creation
// Used when a message arrives via SMTP or other external means
func (c *MyDBConnector) MessageCreated(message imap.Message, literal []byte, mboxIDs []imap.MailboxID) error {
	log.Printf("MessageCreated: Simulating message creation: %s in %d mailboxes", message.ID, len(mboxIDs))

	// In production, you'd save the message to database here
	// Then push update to Gluon
	c.updates <- &imap.MessagesCreated{
		Messages: []*imap.MessageCreated{
			{
				Message:    message,
				Literal:    literal,
				MailboxIDs: mboxIDs,
			},
		},
	}

	return nil
}

// MessagesCreated simulates batch message creation
func (c *MyDBConnector) MessagesCreated(messages []imap.Message, literals [][]byte, mboxIDs [][]imap.MailboxID) error {
	log.Printf("MessagesCreated: Simulating batch creation of %d messages", len(messages))

	// Convert to MessageCreated slice
	messageCreated := make([]*imap.MessageCreated, len(messages))
	for i, msg := range messages {
		messageCreated[i] = &imap.MessageCreated{
			Message:    msg,
			Literal:    literals[i],
			MailboxIDs: mboxIDs[i],
		}
	}

	// Push update to Gluon
	c.updates <- &imap.MessagesCreated{
		Messages: messageCreated,
	}

	return nil
}

// MessageDeleted simulates external message deletion
func (c *MyDBConnector) MessageDeleted(messageID imap.MessageID) error {
	log.Printf("MessageDeleted: Simulating message deletion: %s", messageID)

	// Push update to Gluon
	c.updates <- &imap.MessageDeleted{
		MessageID: messageID,
	}

	return nil
}

// MessageFlagged simulates external flag change (flagged)
func (c *MyDBConnector) MessageFlagged(messageID imap.MessageID, flagged bool) error {
	log.Printf("MessageFlagged: Simulating flagged=%v for message %s", flagged, messageID)

	flags := imap.NewFlagSet()
	if flagged {
		flags = flags.Add(imap.FlagFlagged)
	}

	// Push update to Gluon
	c.updates <- &imap.MessageFlagsUpdated{
		MessageID: messageID,
		Flags:     flags,
	}

	return nil
}

// MessageSeen simulates external flag change (seen)
func (c *MyDBConnector) MessageSeen(messageID imap.MessageID, seen bool) error {
	log.Printf("MessageSeen: Simulating seen=%v for message %s", seen, messageID)

	flags := imap.NewFlagSet()
	if seen {
		flags = flags.Add(imap.FlagSeen)
	}

	// Push update to Gluon
	c.updates <- &imap.MessageFlagsUpdated{
		MessageID: messageID,
		Flags:     flags,
	}

	return nil
}

// MessageRemoved simulates removing a message from a mailbox (for labels)
func (c *MyDBConnector) MessageRemoved(messageID imap.MessageID, mboxID imap.MailboxID) error {
	log.Printf("MessageRemoved: Simulating message %s removed from mailbox %s", messageID, mboxID)

	// Push update to Gluon
	c.updates <- &imap.MessageDeleted{
		MessageID: messageID,
	}
	return nil
}

// MessageUpdated simulates message content update
func (c *MyDBConnector) MessageUpdated(message imap.Message, literal []byte, mboxIDs []imap.MailboxID) error {
	log.Printf("MessageUpdated: Simulating message update: %s", message.ID)

	// Push update to Gluon
	c.updates <- &imap.MessageUpdated{
		Message: message,
	}

	return nil
}

// UIDValidityBumped simulates UID validity bump
func (c *MyDBConnector) UIDValidityBumped() {
	log.Printf("UIDValidityBumped: Simulating UID validity bump")

	// This is typically called internally, not from external events
	// Already implemented in advanced_interfaces.go
}

// ============================================================
// CONFIGURATION METHODS
// ============================================================

// SetAllowMessageCreateWithUnknownMailboxID allows creating messages
// in mailboxes that don't exist yet (useful for some sync scenarios)
func (c *MyDBConnector) SetAllowMessageCreateWithUnknownMailboxID(value bool) {
	log.Printf("SetAllowMessageCreateWithUnknownMailboxID: Setting to %v", value)
	c.allowUnknownMailbox = value
}

// SetFolderPrefix sets the prefix for regular folders
// Example: "INBOX." for nested folders like "INBOX.Sent"
func (c *MyDBConnector) SetFolderPrefix(pfx string) {
	log.Printf("SetFolderPrefix: Setting folder prefix to '%s'", pfx)
	c.folderPrefix = pfx
}

// SetLabelsPrefix sets the prefix for Gmail-style labels
// Example: "[Gmail]/" for labels like "[Gmail]/Sent"
func (c *MyDBConnector) SetLabelsPrefix(pfx string) {
	log.Printf("SetLabelsPrefix: Setting labels prefix to '%s'", pfx)
	c.labelsPrefix = pfx
}

// SetMailboxVisibility sets visibility for a mailbox
// Used to hide/show mailboxes in LIST responses
func (c *MyDBConnector) SetMailboxVisibility(id imap.MailboxID, visibility imap.MailboxVisibility) {
	log.Printf("SetMailboxVisibility: Setting mailbox %s visibility to %v", id, visibility)

	// Update in database
	ctx := context.Background()
	hidden := visibility == imap.Hidden

	_, err := c.db.ExecContext(ctx,
		"UPDATE mailboxes SET hidden = $1 WHERE id = $2 AND user_id = $3",
		hidden, string(id), c.email,
	)

	if err != nil {
		log.Printf("SetMailboxVisibility: Failed to update visibility: %v", err)
	}
}

// SetUpdatesAllowedToFail controls whether update failures are fatal
// If true, failed updates won't stop the connector
func (c *MyDBConnector) SetUpdatesAllowedToFail(value int32) {
	log.Printf("SetUpdatesAllowedToFail: Setting to %v", value)
	c.updatesAllowedToFail = value
}

// Example: When new email arrives via SMTP
func (c *MyDBConnector) OnSMTPMessageReceived(ctx context.Context, rawEmail []byte, recipientMailbox string) error {
	log.Printf("OnSMTPMessageReceived: New email arrived for mailbox %s", recipientMailbox)

	// 1. Save to database
	var mboxID string
	err := c.db.QueryRowContext(ctx,
		"SELECT id FROM mailboxes WHERE name = $1 AND user_id = $2",
		recipientMailbox, c.email,
	).Scan(&mboxID)

	if err != nil {
		return err
	}

	// 2. Create message record
	var msgID string
	err = c.db.QueryRowContext(ctx,
		"INSERT INTO messages (user_id, mailbox_id, raw_content, date, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		c.email, mboxID, rawEmail, context.Background(), context.Background(),
	).Scan(&msgID)

	if err != nil {
		return err
	}

	// 3. Notify Gluon via simulation method
	message := imap.Message{
		ID:    imap.MessageID(msgID),
		Flags: imap.NewFlagSet(),
		Date:  time.Now(),
	}

	return c.MessageCreated(message, rawEmail, []imap.MailboxID{imap.MailboxID(mboxID)})
}

// Example: When webmail marks message as read
func (c *MyDBConnector) OnWebmailMarkAsRead(ctx context.Context, messageID string) error {
	log.Printf("OnWebmailMarkAsRead: Webmail marked message %s as read", messageID)

	// 1. Update database
	_, err := c.db.ExecContext(ctx,
		"UPDATE messages SET seen = true WHERE id = $1 AND user_id = $2",
		messageID, c.email,
	)

	if err != nil {
		return err
	}

	// 2. Notify Gluon
	return c.MessageSeen(imap.MessageID(messageID), true)
}

// Example: When webmail creates new folder
func (c *MyDBConnector) OnWebmailCreateFolder(ctx context.Context, folderName string) error {
	log.Printf("OnWebmailCreateFolder: Webmail created folder %s", folderName)

	// 1. Insert to database
	var mboxID string
	err := c.db.QueryRowContext(ctx,
		"INSERT INTO mailboxes (user_id, name, created_at) VALUES ($1, $2, $3) RETURNING id",
		c.email, folderName, context.Background(),
	).Scan(&mboxID)

	if err != nil {
		return err
	}

	// 2. Notify Gluon
	mailbox := imap.Mailbox{
		ID:   imap.MailboxID(mboxID),
		Name: []string{folderName},
	}

	return c.MailboxCreated(mailbox)
}

// Example: Flush and clear on disconnect
func (c *MyDBConnector) OnDisconnect(ctx context.Context) error {
	log.Printf("OnDisconnect: User disconnecting")

	// Flush any pending operations
	c.Flush()

	// Clear pending updates
	c.ClearUpdates()

	// Close connector
	return c.Close(ctx)
}
