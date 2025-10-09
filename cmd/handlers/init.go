package handlers

import (
	"github.com/enjoys-in/airsend-imap/cmd/services"
)

type Handlers struct {
	AuthHandler *AuthHandler
}

func NewHandlers(svc *services.Services) *Handlers {
	return &Handlers{
		AuthHandler: NewAuthHandler(svc.Auth),
	}
}
