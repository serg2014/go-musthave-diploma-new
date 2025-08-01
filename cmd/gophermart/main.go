package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"errors"

	"github.com/serg2014/go-musthave-diploma/internal/app"
)

func main() {
	a, err := app.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	run_server(a.Address(), a.GetRouter())
}

func run_server(address string, h http.Handler) {
	srv := http.Server{
		Addr:    address,
		Handler: h,
	}

	go func() {
		log.Printf("Start server on %s", address)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Panic("error in ListenAndServe", err)
		}
	}()

	// создаем контекст, который будет отменен при получении сигнала
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 	ждем сигнала от ОС
	<-ctx.Done()
	log.Print("catch signal")

	// даем 5 секунд на завершение
	// TODO время в конфиг
	ctxT, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxT); err != nil {
		log.Printf("Server forced to shutdown: %v\n", err)
	}
	log.Print("Server is shutdown")
}
