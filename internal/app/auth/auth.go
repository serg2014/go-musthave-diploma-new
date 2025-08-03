package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	usercontext "github.com/serg2014/go-musthave-diploma/internal/app/context"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
)

var secretForPassword = []byte("somesecret")
var secretForCookie = []byte("newsomesecret")
var CookieAuthSep = "."
var CookieAuthName = "user_id"
var ErrCookieUserID = fmt.Errorf("no valid cookie %s", CookieAuthName)

func sign(value, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(value)
	return hex.EncodeToString(h.Sum(nil))
}

func SignPassword(password string) string {
	return sign([]byte(password), secretForPassword)
}

func CreateAuthCookie(userID models.UserID) *http.Cookie {
	signature := sign(userID[:], secretForCookie)
	cookie_val := fmt.Sprintf("%s%s%s", userID.String(), CookieAuthSep, signature)

	cookie := &http.Cookie{
		Name:     CookieAuthName,
		Value:    cookie_val,
		Path:     "/",
		HttpOnly: true,                    // Доступ только через HTTP, защита от XSS
		SameSite: http.SameSiteStrictMode, // Защита от CSRF
	}
	return cookie
}

// ====
func checkToken(token string) (*models.UserID, error) {
	items := strings.Split(token, CookieAuthSep)
	if len(items) != 2 {
		return nil, errors.New("bad token")
	}
	userID, err := uuid.Parse(items[0])
	if err != nil {
		return nil, fmt.Errorf("bad userid from cookie: %w", err)
	}
	if sign(userID[:], secretForCookie) != items[1] {
		return nil, errors.New("bad signature")
	}
	return &userID, nil
}

func GetUserIDFromCookie(r *http.Request) (*models.UserID, error) {
	cookie, err := r.Cookie(CookieAuthName)
	if err != nil {
		return nil, ErrCookieUserID
	}
	userID, err := checkToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	return userID, nil
}

func AuthMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := GetUserIDFromCookie(r)
		if err != nil {
			fmt.Printf("no user id from cookie: %v", err)
		}
		if err == nil {
			// сохраним в контекст
			ctx := usercontext.WithUser(r.Context(), userID)
			// TODO может надо Clone
			r2 := r.WithContext(ctx)
			*r = *r2
		}

		// передаём управление хендлеру
		h.ServeHTTP(w, r)
	})
}
