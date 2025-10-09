package services

import (
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/enjoys-in/airsend-imap/cmd/repository"
)

type IMAPService interface {
	CreateMailbox(userID int, name string) error
	DeleteMailbox(userID int, name string) error
	RenameMailbox(userID int, old, new string) error
	ListMailboxes(userID int, subscribed bool) ([]backend.Mailbox, error)
	GetMailbox(userID int, name string) (*backend.Mailbox, error)
	GetMailboxStatus(userID int, name string) (count, unseen uint32, err error)
	CreateMessage(userID int, name string, flags []string, date time.Time, r imap.Literal) error
	ExpungeMessages(userID int, name string, seqset *imap.SeqSet) error
	FetchMessages(userID int, name string, seqset *imap.SeqSet, items []imap.FetchItem) error
	CopyMessages(userID int, src, dest string, seqset *imap.SeqSet) error
}

type IMAP struct {
	IMAPService
}
type imapService struct {
	repo repository.AuthRepository
}

// NewAuthService returns a new instance of the authService, which is a
// UserService implementation. It takes a repository.AuthRepository as a
// parameter and returns a new instance of the authService with the
// given repository.
func NewImapService(repo repository.AuthRepository) IMAPService {
	return &imapService{repo: repo}
}

// CopyMessages copies messages from src to dest mailbox for the given user and sequence set.
func (s *imapService) CopyMessages(userID int, src, dest string, seqset *imap.SeqSet) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// CreateMessage creates a new message in the given mailbox for the given user.
func (s *imapService) CreateMessage(userID int, name string, flags []string, date time.Time, r imap.Literal) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// CreateMailbox creates a new mailbox for the given user.
func (s *imapService) CreateMailbox(userID int, name string) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// DeleteMailbox deletes the given mailbox for the given user.
func (s *imapService) DeleteMailbox(userID int, name string) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// RenameMailbox renames the given mailbox for the given user.
func (s *imapService) RenameMailbox(userID int, old, new string) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// ListMailboxes returns a list of mailboxes for the given user.
func (s *imapService) ListMailboxes(userID int, subscribed bool) ([]backend.Mailbox, error) {
	// TODO: Implement the actual logic using s.repo
	return nil, nil
}

// GetMailbox returns a mailbox for the given user.
func (s *imapService) GetMailbox(userID int, name string) (*backend.Mailbox, error) {
	// TODO: Implement the actual logic using s.repo
	return nil, nil
}

// GetMailboxStatus returns the status of the given mailbox for the given user.
func (s *imapService) GetMailboxStatus(userID int, name string) (count, unseen uint32, err error) {
	// TODO: Implement the actual logic using s.repo
	return 0, 0, nil
}

// ExpungeMessages expunges the given messages for the given mailbox for the given user.
func (s *imapService) ExpungeMessages(userID int, name string, seqset *imap.SeqSet) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}

// FetchMessages fetches the given messages for the given mailbox for the given user.
func (s *imapService) FetchMessages(userID int, name string, seqset *imap.SeqSet, items []imap.FetchItem) error {
	// TODO: Implement the actual logic using s.repo
	return nil
}
