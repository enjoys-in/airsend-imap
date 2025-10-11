package services

import (
	"github.com/enjoys-in/airsend-imap/internal/core/api/repository"
	"github.com/enjoys-in/airsend-imap/internal/interfaces"
)

type Services struct {
	Auth interfaces.AuthService
	IMAP IMAPService
}
type ConcreteServices struct {
	interfaces.Services
}

func NewServices(repo *repository.Repository) *ConcreteServices {
	return &ConcreteServices{
		Services: interfaces.Services{
			Auth: NewAuthService(repo.Auth),
			IMAP: NewImapService(repo.Auth),
		},
	}

}
