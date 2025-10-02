package interfaces

type MailBackend interface {
	Authenticate(username, password string) (userID string, err error)
	ListMailboxes(userID string) ([]MailboxInfo, error)
	FetchMessages(userID, mailbox string, uids []uint32) ([]*Message, error)
	AppendMessage(userID, mailbox string, msg *Message) error
	DeleteMessage(userID, mailbox string, uid uint32) error
}
