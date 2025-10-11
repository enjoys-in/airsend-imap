package repository

import (
	"context"
	"database/sql"
)

type AuthRepository interface {
	FindOne(ctx context.Context, email string) (*User, error)
}

type User struct {
	ID       int
	Name     string
	Email    string
	Password string
}

type UserSettings struct{}
type UserSession struct {
	user     *User
	settings *UserSettings
}
type authRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) AuthRepository {
	return &authRepository{db: db}
}

// FindOne implements AuthRepository.
func (a *authRepository) FindOne(ctx context.Context, email string) (*User, error) {
	row := a.db.QueryRowContext(ctx, "SELECT id, name, email, password FROM mai WHERE email = $1", email)

	var u User
	if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return nil, err
	}
	return &u, nil
}
