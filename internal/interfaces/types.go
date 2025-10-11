package interfaces

import (
	"context"

	"github.com/enjoys-in/airsend-imap/internal/core/api/repository"
)

type AuthService interface {
	GetUser(ctx context.Context, email string) (*repository.User, error)
}

type IMAPService interface {
}

type Services struct {
	Auth AuthService
	IMAP IMAPService
}
