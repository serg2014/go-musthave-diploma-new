package context

import (
	"context"
	"fmt"

	"github.com/serg2014/go-musthave-diploma/internal/app/models"
)

type userCtxKeyType string

var ErrUserIDFromContext = fmt.Errorf("no userid in context")

const userCtxKey userCtxKeyType = "userID"

func WithUser(ctx context.Context, userID *models.UserID) context.Context {
	return context.WithValue(ctx, userCtxKey, userID)
}

func GetUserID(ctx context.Context) (*models.UserID, error) {
	userID, ok := ctx.Value(userCtxKey).(*models.UserID)
	if !ok {
		return nil, ErrUserIDFromContext
	}
	return userID, nil
}
