package services

import (
	"context"

	"github.com/enjoys-in/airsend-imap/internal/core/api/repository"
)

type IMAPService interface {
	FindUserByEmail(ctx context.Context, email string) (*repository.User, error)
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

func (a *imapService) FindUserByEmail(ctx context.Context, email string) (*repository.User, error) {
	return a.repo.FindOne(ctx, email)
}
