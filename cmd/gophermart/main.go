package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"errors"

	"github.com/serg2014/go-musthave-diploma/internal/app"
	"github.com/serg2014/go-musthave-diploma/internal/config"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

func main() {
	cnf, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	if err := logger.Initialize(cnf.LogLevel); err != nil {
		log.Fatal(err)
	}
	a, err := app.NewApp(cnf)
	if err != nil {
		logger.Log.Fatal("error NewApp", zap.Error(err))
	}

	run_server(a.Address(), a.GetRouter())
}

func run_server(address string, h http.Handler) {
	srv := http.Server{
		Addr:    address,
		Handler: h,
	}

	go func() {
		logger.Log.Info(fmt.Sprintf("Start server on %s", address))
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Panic("error in ListenAndServe", zap.Error(err))
		}
		logger.Log.Info("Server is shutdown")
	}()

	// создаем контекст, который будет отменен при получении сигнала
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 	ждем сигнала от ОС
	<-ctx.Done()
	logger.Log.Info("catch signal")

	// даем 5 секунд на завершение
	// TODO время в конфиг
	ctxT, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxT); err != nil {
		logger.Log.Info("Server forced to shutdown", zap.Error(err))
	}
	logger.Log.Info("Finish")
}
