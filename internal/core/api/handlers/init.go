package handlers

import (
	"github.com/enjoys-in/airsend-imap/internal/core/api/services"
)

type Handlers struct {
	AuthHandler *AuthHandler
}

func NewHandlers(svc *services.ConcreteServices) *Handlers {
	return &Handlers{
		AuthHandler: NewAuthHandler(svc),
	}
}
