package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	usercontext "github.com/serg2014/go-musthave-diploma/internal/app/context"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

var secretForPassword = []byte("somesecret")
var secretForCookie = []byte("newsomesecret")

const (
	cookieAuthName   = "user_id"
	cookieExpireTime = 2 * time.Hour
)

var ErrCookieUserID = fmt.Errorf("no valid cookie %s", cookieAuthName)
var ErrCookieExpired = fmt.Errorf("cookie expired")
var ErrBadToken = errors.New("bad token")
var ErrBadSignature = errors.New("bad signature")

func sign(value, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(value)
	return hex.EncodeToString(h.Sum(nil))
}

func SignPassword(password string) string {
	return sign([]byte(password), secretForPassword)
}

func createToken(userID models.UserID, now time.Time) string {
	// userid + rnd + expireTime = 16 + 4 + 8 = 28
	b := make([]byte, 28)
	copy(b[0:16], userID[:])

	c := b[16:20]
	rand.Read(c)

	t := uint64(now.Add(cookieExpireTime).Unix())
	binary.BigEndian.PutUint64(b[20:28], t)

	// signature - 32 bytes
	signature := sign(b, secretForCookie)
	return fmt.Sprintf("%x%s", b, signature)
}

func checkToken(token string, now time.Time) (*models.UserID, error) {
	// userid + rnd + expireTime + signature = 16 + 4 + 8 + 32 = 60 bytes
	if len(token) != 120 {
		return nil, ErrBadToken
	}
	signature := token[56:120]

	data, err := hex.DecodeString(token[0:56])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadToken, err)
	}

	if sign(data[0:28], secretForCookie) != signature {
		return nil, ErrBadSignature
	}

	expireTime := time.Unix(int64(binary.BigEndian.Uint64(data[20:28])), 0)
	if now.After(expireTime) {
		return nil, ErrCookieExpired
	}
	userID, err := uuid.FromBytes(data[0:16])
	if err != nil {
		logger.Log.Error("can not parse userid from valid cookie", zap.Error(err))
		return nil, fmt.Errorf("bad userid from cookie: %w", err)
	}

	return &userID, nil
}

func CreateAuthCookie(userID models.UserID) *http.Cookie {
	cookieVal := createToken(userID, time.Now())
	cookie := &http.Cookie{
		Name:     cookieAuthName,
		Value:    cookieVal,
		Path:     "/",
		HttpOnly: true,                    // Доступ только через HTTP, защита от XSS
		SameSite: http.SameSiteStrictMode, // Защита от CSRF
		Expires:  time.Now().Add(cookieExpireTime),
	}
	return cookie
}

func GetUserIDFromCookie(r *http.Request) (*models.UserID, error) {
	cookie, err := r.Cookie(cookieAuthName)
	if err != nil {
		return nil, ErrCookieUserID
	}
	userID, err := checkToken(cookie.Value, time.Now())
	if err != nil {
		return nil, err
	}
	return userID, nil
}

func AuthMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := usercontext.GetUserID(r.Context())
		if err != nil {
			code := http.StatusUnauthorized
			http.Error(w, http.StatusText(code), code)
			return
		}

		// передаём управление хендлеру
		h.ServeHTTP(w, r)
	})
}

func WithUserMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := GetUserIDFromCookie(r)
		if err != nil {
			logger.Log.Debug("no user id from cookie", zap.Error(err))
		}
		// rwu - request with user
		rwu := r
		if err == nil {
			// сохраним в контекст
			ctx := usercontext.WithUser(r.Context(), userID)
			rwu = r.WithContext(ctx)
		}

		// передаём управление хендлеру
		h.ServeHTTP(w, rwu)
	})
}
