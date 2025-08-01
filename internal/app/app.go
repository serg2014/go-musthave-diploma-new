package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/serg2014/go-musthave-diploma/internal/config"
)

type App struct {
	config *config.Config
	router *chi.Mux
}

func NewApp() (*App, error) {
	cnf, err := config.NewConfig()
	if err != nil {
		//log.Fatal(err)
		return nil, err
	}
	app := &App{
		config: cnf,
		router: chi.NewRouter(),
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
