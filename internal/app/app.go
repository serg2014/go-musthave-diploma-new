package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/app/storage"
	"github.com/serg2014/go-musthave-diploma/internal/config"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

const ChanLimit = 100

func generateWho(port uint16) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%x%x", time.Now().Unix(), port)
	randLength := 4
	part2 := make([]byte, randLength)
	rand.Read(part2)
	b.WriteString(hex.EncodeToString(part2))
	// len(b.String()) = 4 + 2 + 4(randLength) = 10
	return b.String()
}

type App struct {
	config  *config.Config
	router  *chi.Mux
	store   storage.Storager
	reqChan chan *models.ProcessingOrderItem
	resChan chan *models.AccrualOrderItem
	who     string
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
		who:     generateWho(cnf.Port),
	}
	app.setRoute()
	logger.Log.Debug("app create", zap.String("who", app.who))
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
	// TODO количество воркеров в конфиг
	for i := range 10 {
		go a.worker(ctx, i)
	}

	cleanup := func() {
		err := a.store.CleanOrdersForProcess(context.Background(), a.who)
		if err != nil {
			logger.Log.Error("failed CleanOrdersForProcess", zap.Error(err))
		}
	}
	defer cleanup()

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
			data, err := a.store.GetOrdersForProcess(ctx, a.who, ChanLimit)
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
				err := a.store.UpdateOrders(ctx, accrual, a.who)
				if err != nil {
					logger.Log.Error("failed UpdateOrders", zap.Error(err))
				}
			}
		}
	}
}
