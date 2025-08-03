package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/serg2014/go-musthave-diploma/internal/app/storage"
	"github.com/serg2014/go-musthave-diploma/internal/config"
)

type App struct {
	config *config.Config
	router *chi.Mux
	store  storage.Storager
}

func NewApp(cnf *config.Config) (*App, error) {
	s, err := storage.NewStorage(context.Background(), cnf.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("filed to create NewStorage: %w", err)
	}
	app := &App{
		config: cnf,
		router: chi.NewRouter(),
		store:  s,
	}
	app.setRoute()
	return app, nil
}

func (a *App) Address() string {
	return a.config.Address
}

func (a *App) GetRouter() *chi.Mux {
	return a.router
}

func checkLuhn(code string) error {
	_, err := strconv.Atoi(code)
	if err != nil {
		return errors.New("not digit")
	}

	sum := 0
	parity := len(code) % 2
	for i := 0; i < len(code); i++ {
		digit, _ := strconv.Atoi(string(code[i]))
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	if sum%10 != 0 {
		return errors.New("bad check")
	}
	return nil
}

/*
 function checkLuhn(string purportedCC) {
     int sum := 0
     int nDigits := length(purportedCC)
     int parity := nDigits modulus 2
     for i from 0 to nDigits - 1 {
         int digit := integer(purportedCC[i])
         if i modulus 2 = parity
             digit := digit × 2
             if digit > 9
                 digit := digit - 9
         sum := sum + digit
     }
     return (sum modulus 10) = 0
 }
*/
