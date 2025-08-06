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
		// TODO const or conf
		period := 2 * time.Hour
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

		// даем 5 секунд на завершение
		// TODO время в конфиг
		ctxT, cancelT := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelT()
		if err := srv.Shutdown(ctxT); err != nil {
			logger.Log.Info("Server forced to shutdown", zap.Error(err))
		}
	}()

	logger.Log.Info(fmt.Sprintf("Start server on %s", a.Address()))
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Log.Panic("error in ListenAndServe", zap.Error(err))
	}

	// отменяем контекст, чтобы завершить горутины
	cancel()

	wg.Wait()
	logger.Log.Info("Server is shutdown")

	/*
		// другой подход, тоже рабочий
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
	*/
}
