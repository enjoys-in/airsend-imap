package imap

import (
	"context"
	"github.com/ProtonMail/gluon/imap"

	"time"
)

type Connector interface {
	Authorize(ctx context.Context, username string, password []byte) bool
	CreateMailbox(ctx context.Context, name []string) (imap.Mailbox, error)
	GetMessageLiteral(ctx context.Context, id imap.MessageID) ([]byte, error)
	GetMailboxVisibility(ctx context.Context, mboxID imap.MailboxID) imap.MailboxVisibility
	UpdateMailboxName(ctx context.Context, mboxID imap.MailboxID, newName []string) error
	DeleteMailbox(ctx context.Context, mboxID imap.MailboxID) error
	CreateMessage(ctx context.Context, mboxID imap.MailboxID, literal []byte, flags imap.FlagSet, date time.Time) (imap.Message, []byte, error)
	AddMessagesToMailbox(ctx context.Context, messageIDs []imap.MessageID, mboxID imap.MailboxID) error
	RemoveMessagesFromMailbox(ctx context.Context, messageIDs []imap.MessageID, mboxID imap.MailboxID) error
	MoveMessages(ctx context.Context, messageIDs []imap.MessageID, mboxFromID, mboxToID imap.MailboxID) (bool, error)
	MarkMessagesSeen(ctx context.Context, messageIDs []imap.MessageID, seen bool) error
	MarkMessagesFlagged(ctx context.Context, messageIDs []imap.MessageID, flagged bool) error
	GetUpdates() <-chan imap.Update
	Close(ctx context.Context) error
}
type Sync interface {
	Sync() // Periodic sync
}

type MailboxReadOps interface {
	GetMailboxMessages()     // SELECT/EXAMINE
	GetMailboxMessageCount() // STATUS
}

type MailboxWriteOps interface {
	Expunge() // EXPUNGE
}
