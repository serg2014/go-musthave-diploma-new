package app

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
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

// должен быть согласован с лимитом в update
const ChanLimit = 100

var ErrLuhnNotDigit = errors.New("not digit")
var ErrLuhn = errors.New("bad check")

func generateWho(port uint16) string {
	// time + rnd + port = 8 + 4 + 2 = 14
	b := make([]byte, 14)
	t := uint64(time.Now().Unix())
	binary.BigEndian.PutUint64(b[0:8], t)

	c := b[8:12]
	rand.Read(c)

	binary.BigEndian.PutUint16(b[12:14], port)
	return hex.EncodeToString(b)
}

type App struct {
	config  *config.Config
	router  *chi.Mux
	store   storage.Storager
	reqChan chan *models.ProcessingOrderItem
	resChan chan *models.AccrualOrderItem
	whoLock string
}

func NewApp(cnf *config.Config) (*App, error) {
	s, err := storage.NewStorage(context.Background(), cnf.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("filed to create NewStorage: %w", err)
	}
	app := &App{
		config:  cnf,
		router:  chi.NewRouter(),
		store:   s,
		reqChan: make(chan *models.ProcessingOrderItem, ChanLimit),
		resChan: make(chan *models.AccrualOrderItem, ChanLimit),
		whoLock: generateWho(cnf.Port),
	}
	app.setRoute()
	logger.Log.Debug("app create", zap.String("who", app.whoLock))
	return app, nil
}

func (a *App) Address() string {
	return a.config.Address
}

func (a *App) AccrualAddress() string {
	return a.config.AccrualAddress
}

func (a *App) GetRouter() *chi.Mux {
	return a.router
}

func (a *App) GetShutdownTimeout() time.Duration {
	return a.config.ShutdownTimeout
}

func (a *App) GetCleanupAfterCrashDuration() time.Duration {
	return a.config.CleanupAfterCrashDuration
}

func checkLuhn(code string) error {
	_, err := strconv.Atoi(code)
	if err != nil {
		return ErrLuhnNotDigit
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
		return ErrLuhn
	}
	return nil
}

func (a *App) CleanupAfterCrash(ctx context.Context, t time.Duration) error {
	err := a.store.CleanupAfterCrash(ctx, t)
	return err
}

func (a *App) ProcessOrders(ctx context.Context) {
	for i := range a.config.WorkerCount {
		go a.worker(ctx, i)
	}

	cleanup := func() {
		err := a.store.CleanOrdersForProcess(context.Background(), a.whoLock)
		if err != nil {
			logger.Log.Error("failed CleanOrdersForProcess", zap.Error(err))
		}
	}
	defer cleanup()

	ticker := time.NewTicker(a.config.OrdersForProcessDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
			data, err := a.store.GetOrdersForProcess(ctx, a.whoLock, ChanLimit)
			if err != nil {
				logger.Log.Error("failed GetOrdersForProcess", zap.Error(err))
				break
			}
			if len(data) != 0 {
				for i := range data {
					// send orderid and userid
					a.reqChan <- &data[i]
				}

				accrual := make([]*models.AccrualOrderItem, 0, len(data))
				for range data {
					select {
					case <-ctx.Done():
						return
					case itemPtr := <-a.resChan:
						if itemPtr.Error != nil {
							logger.Log.Debug(
								"failed get Accrual",
								zap.Error(itemPtr.Error),
								zap.String("orderID", itemPtr.OrderID),
							)
						} else {
							accrual = append(accrual, itemPtr)
						}
					}
				}
				err := a.store.UpdateOrders(ctx, accrual, a.whoLock)
				if err != nil {
					logger.Log.Error("failed UpdateOrders", zap.Error(err))
				}
			}
		}
	}
}
