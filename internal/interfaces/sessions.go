package interfaces

import "sync"

type Session struct {
	UserID      string
	SelectedBox string
	uidManager  *UIDManager
	flags       map[uint32][]string
	mu          sync.RWMutex // Thread-safe access
}

func NewSession(userID string) *Session {
	return &Session{
		UserID:     userID,
		uidManager: NewUIDManager(),
		flags:      make(map[uint32][]string),
		mu:         sync.RWMutex{},
	}
}
