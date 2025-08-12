package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckToken(t *testing.T) {
	userID, err := uuid.Parse("c771b3eb-7e54-4509-8213-a79c211920f0")
	require.NoError(t, err)

	now := time.Date(2025, time.August, 13, 1, 46, 0, 0, time.UTC)
	expiredToken := createToken(userID, now.Add(-1*(cookieExpireTime+time.Second)))

	tests := []struct {
		name  string
		now   time.Time
		token func() *string
		user  models.UserID
		err   error
	}{
		{
			name:  "create parse",
			now:   now,
			token: func() *string { return nil },
			user:  userID,
		},
		{
			name:  "short token",
			now:   now,
			token: func() *string { t := ""; return &t },
			user:  userID,
			err:   ErrBadToken,
		},
		{
			name:  "bad token. not hex",
			now:   now,
			token: func() *string { t := strings.Repeat("q", 120); return &t },
			user:  userID,
			err:   ErrBadToken,
		},
		{
			name:  "bad signature",
			now:   now,
			token: func() *string { t := strings.Repeat("a", 120); return &t },
			user:  userID,
			err:   ErrBadSignature,
		},
		{
			name:  "expired token",
			now:   now,
			token: func() *string { return &expiredToken },
			user:  userID,
			err:   ErrCookieExpired,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokenPtr := test.token()
			if tokenPtr == nil {
				token := createToken(userID, test.now)
				tokenPtr = &token
			}

			userIDPtr, err := checkToken(*tokenPtr, test.now)
			if test.err != nil {
				require.ErrorIs(t, err, test.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, userID, *userIDPtr)
		})
	}
}
