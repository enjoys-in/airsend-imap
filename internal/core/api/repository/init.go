package repository

import (
	plugins "github.com/enjoys-in/airsend-imap/internal/plugins/postgres"
)

type Repository struct {
	Auth AuthRepository
}

func NewRepository(db *plugins.DB) *Repository {
	return &Repository{
		Auth: NewAuthRepository(db.Conn),
	}
}
