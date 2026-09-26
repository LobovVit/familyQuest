package application

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// PlatformGateway separates account routing from family operations.
// PlatformGateway отделяет вход аккаунта от семейных операций.
type PlatformGateway interface {
	Account(context.Context, string, string) (int64, int64, error)
	Subscription(context.Context, int64) (domain.FamilySubscription, error)
}
