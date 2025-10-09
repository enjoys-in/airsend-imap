package services

import "github.com/enjoys-in/airsend-imap/cmd/repository"

type Services struct {
	Auth AuthService
	IMAP IMAPService
}

func NewServices(repo *repository.Repository) *Services {
	return &Services{
		Auth: NewAuthService(repo.Auth),
		IMAP: NewImapService(repo.Auth),
	}
}
