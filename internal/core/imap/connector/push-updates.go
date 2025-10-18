package connector

import (
	"github.com/ProtonMail/gluon/imap"
)

// ============================================================
// UPDATE HELPERS
// Methods to push various update types to Gluon
// ============================================================

// PushMailboxCreated notifies Gluon about new mailbox
func (c *MyDBConnector) PushMailboxCreated(mbox imap.Mailbox) {
	c.Updates <- &imap.MailboxCreated{
		Mailbox: mbox,
	}
}

// PushMailboxDeleted notifies Gluon about deleted mailbox
func (c *MyDBConnector) PushMailboxDeleted(mboxID imap.MailboxID) {
	c.Updates <- &imap.MailboxDeleted{
		MailboxID: mboxID,
	}
}

// // PushMailboxRenamed notifies Gluon about renamed mailbox
// func (c *MyDBConnector) PushMailboxRenamed(mboxID imap.MailboxID, newName []string) {
// 	c.Updates <- &imap.MailboxRenamed{
// 		MailboxID: mboxID,
// 		NewName:   newName,
// 	}

// }

// PushMessagesCreated notifies Gluon about new messages
// func (c *MyDBConnector) PushMessagesCreated(messages []imap.Message) {
// 	c.Updates <- &imap.MessagesCreated{
// 		Messages: messages,
// 	}
// }

// PushMessageDeleted notifies Gluon about deleted message
func (c *MyDBConnector) PushMessageDeleted(messageID imap.MessageID) {
	c.Updates <- &imap.MessageDeleted{
		MessageID: messageID,
	}
}

// PushMessageFlagsChanged notifies Gluon about flag changes
// func (c *MyDBConnector) PushMessageFlagsChanged(messageID imap.MessageID, flags imap.FlagSet) {
// 	c.Updates <- &imap.MessageFlagsChanged{
// 		MessageID: messageID,
// 		Flags:     flags,
// 	}
// }

// // PushMessageMoved notifies Gluon about moved message
// func (c *MyDBConnector) PushMessageMoved(messageID imap.MessageID, fromMbox, toMbox imap.MailboxID) {
// 	c.Updates <- &imap.MessageMoved{
// 		MessageID:     messageID,
// 		FromMailboxID: fromMbox,
// 		ToMailboxID:   toMbox,
// 	}
// }

// ============================================================
// HELPER FUNCTIONS
// ============================================================

// extractHeaders extracts headers from raw email content
func extractHeaders(rawContent []byte) []byte {
	// Find double newline that separates headers from body
	for i := 0; i < len(rawContent)-3; i++ {
		if rawContent[i] == '\r' && rawContent[i+1] == '\n' &&
			rawContent[i+2] == '\r' && rawContent[i+3] == '\n' {
			return rawContent[:i+4]
		}
		if rawContent[i] == '\n' && rawContent[i+1] == '\n' {
			return rawContent[:i+2]
		}
	}
	return rawContent
}

// extractMIMESection extracts a specific MIME section
func extractMIMESection(rawContent []byte, section string) []byte {
	// This is a simplified placeholder
	// In production, use a proper MIME parser like:
	// - github.com/emersion/go-message
	// - net/mail package

	switch section {
	case "TEXT":
		// Return body only (after headers)
		headers := extractHeaders(rawContent)
		return rawContent[len(headers):]
	case "HEADER":
		return extractHeaders(rawContent)
	default:
		// For numbered sections like "1", "1.1", parse MIME structure
		// This requires proper MIME multipart parsing
		return rawContent
	}
}
