package connector

import (
	"sync"
	"time"

	"github.com/ProtonMail/gluon/imap"
	"github.com/bradenaw/juniper/xslices"
	"golang.org/x/exp/maps"
)

func NewUIDValidityGenerator() imap.UIDValidityGenerator {

	return imap.DefaultEpochUIDValidityGenerator()
}

type MailboxState struct {
	flags, permFlags, attrs imap.FlagSet

	messages   map[imap.MessageID]*MessageState
	mailboxes  map[imap.MailboxID]*MailboxOptions
	lastIMAPID imap.IMAPID

	lock sync.RWMutex
}
type MessageState struct {
	literal   []byte
	parsed    *imap.ParsedMessage
	seen      bool
	flagged   bool
	forwarded bool
	date      time.Time
	flags     imap.FlagSet

	mboxIDs map[imap.MailboxID]struct{}
}

func newMailboxState(flags, permFlags, attrs imap.FlagSet) *MailboxState {
	return &MailboxState{
		flags:      flags,
		permFlags:  permFlags,
		attrs:      attrs,
		messages:   make(map[imap.MessageID]*MessageState),
		mailboxes:  make(map[imap.MailboxID]*MailboxOptions),
		lastIMAPID: imap.NewIMAPID(),
	}
}

type MailboxOptions struct {
	id        imap.MailboxID
	name      []string
	exclusive bool
}

func (state *MailboxState) createMailbox(id imap.MailboxID, name []string, exclusive bool) imap.Mailbox {
	state.lock.Lock()
	defer state.lock.Unlock()

	mboxID := imap.MailboxID(id)

	state.mailboxes[mboxID] = &MailboxOptions{
		name:      name,
		exclusive: exclusive,
		id:        mboxID,
	}

	return state.toMailbox(mboxID)
}

func (state *MailboxState) toMailbox(mboxID imap.MailboxID) imap.Mailbox {
	return imap.Mailbox{
		ID:             mboxID,
		Name:           state.mailboxes[mboxID].name,
		Flags:          state.flags,
		PermanentFlags: state.permFlags,
		Attributes:     state.attrs,
	}
}
func (state *MailboxState) getMailboxes() []imap.Mailbox {
	state.lock.Lock()
	defer state.lock.Unlock()

	return xslices.Map(maps.Keys(state.mailboxes), func(mboxID imap.MailboxID) imap.Mailbox {
		return state.toMailbox(mboxID)
	})
}

// func (state *MailboxState) getMessages() []imap.Message {
// 	state.lock.Lock()
// 	defer state.lock.Unlock()

//		return xslices.Map(maps.Keys(state.messages), func(messageID imap.MessageID) imap.Message {
//			return state.toMessage(messageID)
//		})
//	}
func (state *MailboxState) getMailbox(mboxID imap.MailboxID) (imap.Mailbox, error) {
	state.lock.Lock()
	defer state.lock.Unlock()

	if _, ok := state.mailboxes[mboxID]; !ok {
		return imap.Mailbox{}, ErrNoSuchMailbox
	}

	return state.toMailbox(mboxID), nil
}
