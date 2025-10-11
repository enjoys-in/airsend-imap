package services

import (
	"context"

	"github.com/enjoys-in/airsend-imap/internal/core/api/repository"
	"github.com/enjoys-in/airsend-imap/internal/interfaces"
)

//	type AuthService interface {
//		GetUser(ctx context.Context, email string) (*repository.User, error)
//	}
type authService struct {
	repo repository.AuthRepository
}

// NewAuthService returns a new instance of the authService, which is a
// UserService implementation. It takes a repository.AuthRepository as a
// parameter and returns a new instance of the authService with the
// given repository.
func NewAuthService(repo repository.AuthRepository) interfaces.AuthService {
	return &authService{repo: repo}
}

// GetUser retrieves a user by their email address. It takes a context and
// an email address as parameters and returns a pointer to a repository.User
// and an error. If the user is found, the returned error is nil. If
// the user is not found, the returned error is not nil.
func (a *authService) GetUser(ctx context.Context, email string) (*repository.User, error) {
	return a.repo.FindOne(ctx, email)
}
