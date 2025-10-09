package imap

import (
	"errors"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/enjoys-in/airsend-imap/cmd/services"
)

// ------------------- Backend -------------------

// backendImpl implements backend.Backend
type backendImpl struct {
	svc *services.Services
}

// NewBackend returns a backend.Backend interface
func NewBackend(svc *services.Services) backend.Backend {
	return &backendImpl{svc: svc}
}

// Login authenticates a user (implements backend.Backend)
func (b *backendImpl) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {
	user, err := b.svc.Auth.GetUser(nil, username)
	if err != nil || user.Password != password {
		return nil, errors.New("invalid username or password")
	}

	return &userImpl{
		ID:   user.ID,
		Name: username,
		svc:  b.svc,
	}, nil
}

// ------------------- User -------------------

type userImpl struct {
	ID   int
	Name string
	svc  *services.Services
}

// Username implements backend.User.
func (u *userImpl) Username() string {
	return u.Name
}

// Logout implements backend.User.
func (u *userImpl) Logout() error {
	// Optional cleanup logic
	return nil
}

// CreateMailbox implements backend.User.
func (u *userImpl) CreateMailbox(name string) error {
	return u.svc.IMAP.CreateMailbox(u.ID, name)
}

// DeleteMailbox implements backend.User.
func (u *userImpl) DeleteMailbox(name string) error {
	return u.svc.IMAP.DeleteMailbox(u.ID, name)
}

// RenameMailbox implements backend.User.
func (u *userImpl) RenameMailbox(existingName string, newName string) error {
	return u.svc.IMAP.RenameMailbox(u.ID, existingName, newName)
}

// ListMailboxes implements backend.User.
func (u *userImpl) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	mailboxes, err := u.svc.IMAP.ListMailboxes(u.ID, subscribed)
	if err != nil {
		return nil, err
	}

	result := make([]backend.Mailbox, 0, len(mailboxes))
	for _, m := range mailboxes {
		result = append(result, &mailboxImpl{
			userID: u.ID,
			svc:    u.svc,
			name:   m.Name(),
		})
	}
	return result, nil
}

// GetMailbox implements backend.User.
func (u *userImpl) GetMailbox(name string) (backend.Mailbox, error) {
	_, err := u.svc.IMAP.GetMailbox(u.ID, name)
	if err != nil {
		return nil, err
	}

	return &mailboxImpl{
		userID: u.ID,
		svc:    u.svc,
		name:   name,
	}, nil
}

// ------------------- Mailbox -------------------

type mailboxImpl struct {
	userID int
	svc    *services.Services
	name   string
}

// Check implements backend.Mailbox.
func (m *mailboxImpl) Check() error {
	panic("unimplemented")
}

// CopyMessages implements backend.Mailbox.
func (m *mailboxImpl) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	panic("unimplemented")
}

// CreateMessage implements backend.Mailbox.
func (m *mailboxImpl) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	panic("unimplemented")
}

// Expunge implements backend.Mailbox.
func (m *mailboxImpl) Expunge() error {
	panic("unimplemented")
}

// Info implements backend.Mailbox.
func (m *mailboxImpl) Info() (*imap.MailboxInfo, error) {
	panic("unimplemented")
}

// ListMessages implements backend.Mailbox.
func (m *mailboxImpl) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	panic("unimplemented")
}

// Name implements backend.Mailbox.
func (m *mailboxImpl) Name() string {
	panic("unimplemented")
}

// SearchMessages implements backend.Mailbox.
func (m *mailboxImpl) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	panic("unimplemented")
}

// SetSubscribed implements backend.Mailbox.
func (m *mailboxImpl) SetSubscribed(subscribed bool) error {
	panic("unimplemented")
}

// Status implements backend.Mailbox.
func (m *mailboxImpl) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	panic("unimplemented")
}

// UpdateMessagesFlags implements backend.Mailbox.
func (m *mailboxImpl) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error {
	panic("unimplemented")
}
