package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	srv := http.Server{
		Addr:    a.Address(),
		Handler: a.GetRouter(),
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		period := a.GetCleanupAfterCrashDuration()
		ticker := time.NewTicker(period)
		for {
			err := a.CleanupAfterCrash(ctx, period)
			if err != nil {
				logger.Log.Error("failed cleanup", zap.Error(err))
			} else {
				logger.Log.Debug("cleanup ok")
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				logger.Log.Info("Stop cleanup goroutine")
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		a.ProcessOrders(ctx)
		logger.Log.Info("Stop processed goroutine")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// создаем контекст, который будет отменен при получении сигнала
		ctxS, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		select {
		// 	ждем сигнала от ОС
		case <-ctxS.Done():
			logger.Log.Info("catch signal")
		// ждем отмены контекста
		case <-ctx.Done():
			logger.Log.Info("stop")
		}

		ctxT, cancelT := context.WithTimeout(context.Background(), a.GetShutdownTimeout())
		defer cancelT()
		if err := srv.Shutdown(ctxT); err != nil {
			logger.Log.Info("Server forced to shutdown", zap.Error(err))
		}
	}()

	logger.Log.Info(fmt.Sprintf("Start server on %s use accrual service: %s", a.Address(), a.AccrualAddress()))
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Log.Panic("error in ListenAndServe", zap.Error(err))
	}

	// отменяем контекст, чтобы завершить горутины
	cancel()

	wg.Wait()
	logger.Log.Info("Server is shutdown")
}
