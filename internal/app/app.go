package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/app/storage"
	"github.com/serg2014/go-musthave-diploma/internal/config"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

const ChanLimit = 100

type App struct {
	config  *config.Config
	router  *chi.Mux
	store   storage.Storager
	reqChan chan *models.ProcessingOrderItem
	resChan chan *models.AccrualOrderItem
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
		// TODO должен быть согласован с лимитом в update
		reqChan: make(chan *models.ProcessingOrderItem, ChanLimit),
		resChan: make(chan *models.AccrualOrderItem, ChanLimit),
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

func (a *App) CleanupAfterCrash(ctx context.Context, t time.Duration) error {
	err := a.store.CleanupAfterCrash(ctx, t)
	return err
}

func (a *App) ProcessOrders(ctx context.Context) {
	// запустить N воркеров читающих из a.reqChan и пишущих в a.resChan

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := a.store.GetOrdersForProcess(ctx, ChanLimit)
			if err != nil {
				logger.Log.Error("failed GetOrdersForProcess", zap.Error(err))
				break
			}
			if len(data) != 0 {
				for i := range data {
					// send orderid and userid
					a.reqChan <- &data[i]
				}

				// заказы по которым получили терминальный статус
				//finish := make([]*models.AccrualOrderItem, 0, len(data))
				// заказы по которым не получили терминальный статус
				//processing := make([]*models.AccrualOrderItem, 0, len(data))
				accrual := make([]*models.AccrualOrderItem, len(data))
				for i := range accrual {
					select {
					case <-ctx.Done():
						return
					case itemPtr := <-a.resChan:
						/*
							if slices.Contains(models.AccrualOrderTerminateStatus, itemPtr.Status) {
								finish = append(finish, itemPtr)
							} else {
								processing = append(processing, itemPtr)
							}
						*/
						accrual[i] = itemPtr
					}
				}
				err := a.store.UpdateOrders(ctx, accrual)
				if err != nil {
					logger.Log.Error("failed UpdateOrders", zap.Error(err))
					return
				}
			}
		}
	}
}
