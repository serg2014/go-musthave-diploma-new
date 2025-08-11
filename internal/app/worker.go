package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

var (
	ErrHTTPNoContent           = errors.New("http 204")
	ErrHTTPNTooManyRequets     = errors.New("http 429")
	ErrHTTPInternalServerError = errors.New("http 500")
	ErrHTTPOther               = errors.New("http other")
	ErrTimeout                 = errors.New("timeout")
	ErrContext                 = errors.New("error context")
	ErrDoneContext             = errors.New("done context")
)

func geturlWithRetries(ctx context.Context, client *http.Client, url string) (*models.AccrualOrderItem, error) {
	retry := []time.Duration{
		1500 * time.Millisecond,
		3000 * time.Millisecond,
		0 * time.Millisecond,
	}

	var (
		data *models.AccrualOrderItem
		err  error
	)
	for _, dur := range retry {
		data, err = geturl(ctx, client, url)
		if err == nil {
			break
		}
		// TODO как поймать timeout?
		// in geturl err: Get "http://localhost:8080/api/orders/32": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
		// data: <nil> error: failed get
		// data: <nil> error: bad json: EOF - нет тела
		// тут таймаут на получении тела
		// data: <nil> error: bad json: context deadline exceeded (Client.Timeout or context cancellation while reading body)
		if errors.Is(err, ErrTimeout) ||
			errors.Is(err, ErrHTTPInternalServerError) ||
			errors.Is(err, ErrHTTPNTooManyRequets) {
			timeout := time.After(dur)
			select {
			case <-timeout:
				continue
			case <-ctx.Done():
				return nil, ErrDoneContext
			}
		}
		return nil, err
	}

	return data, err
}

func geturl(ctx context.Context, client *http.Client, url string) (*models.AccrualOrderItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrContext
	}
	response, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("failed get: %w", err)
	}
	defer response.Body.Close()

	statusToError := map[int]error{
		http.StatusOK:                  nil,
		http.StatusNoContent:           ErrHTTPNoContent,
		http.StatusTooManyRequests:     ErrHTTPNTooManyRequets,
		http.StatusInternalServerError: ErrHTTPInternalServerError,
	}
	err, ok := statusToError[response.StatusCode]
	if !ok {
		err = ErrHTTPOther
	}
	if err != nil {
		return nil, err
	}

	data := models.AccrualOrderItem{}
	dec := json.NewDecoder(response.Body)
	if err := dec.Decode(&data); err != nil {
		if os.IsTimeout(err) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("bad json: %w", err)
	}

	return &data, nil
}

func (a *App) worker(ctx context.Context, i int) {
	for {
		select {
		case data := <-a.reqChan:
			resp := a.getAccrual(data)
			a.resChan <- resp
		case <-ctx.Done():
			logger.Log.Debug("Stop worker", zap.Int("num", i))
			return
		}
	}
}

func (a *App) getAccrual(item *models.ProcessingOrderItem) *models.AccrualOrderItem {
	endpoint := fmt.Sprintf("%s/api/orders/%s", a.AccrualAddress(), item.OrderID)
	client := &http.Client{
		// TODO timeout в конфиг
		Timeout: 5 * time.Second,
	}
	data, err := geturlWithRetries(context.Background(), client, endpoint)
	if err != nil {
		return &models.AccrualOrderItem{
			OrderID: item.OrderID,
			UserID:  item.UserID,
			Error:   err,
		}
	}

	return data
}
