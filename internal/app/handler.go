package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/serg2014/go-musthave-diploma/internal/app/auth"
	usercontext "github.com/serg2014/go-musthave-diploma/internal/app/context"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/app/storage"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

func (a *App) setRoute() {
	r := a.GetRouter()
	r.Use(auth.WithUserMiddleware)
	r.Use(logger.WithLogging)
	r.Post("/api/user/register", a.registerUser())
	r.Post("/api/user/login", a.authUser())

	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		//r.Use(middleware.Recoverer)

		r.Route("/api/user", func(r chi.Router) {
			r.Post("/orders", a.createOrder())
			r.Get("/orders", a.GetOrders())
			r.Get("/balance", a.Balance())
			r.Post("/balance/withdraw", a.Withdraw())
		})
	})
}

func simpleError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

func (a *App) registerUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterUser
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Login == "" || req.Password == "" {
			http.Error(w, "empty login or password", http.StatusBadRequest)
			return
		}
		hashPassword := auth.SignPassword(req.Password)
		userIDPtr, err := a.store.CreateUser(r.Context(), req.Login, hashPassword)
		if err != nil {
			if errors.Is(err, storage.ErrUserExists) {
				http.Error(w, "user exists", http.StatusConflict)
				return
			}
			simpleError(w, http.StatusInternalServerError)
			return
		}
		setAuthCookie(*userIDPtr, w)
	}
}

func (a *App) authUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterUser
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Login == "" || req.Password == "" {
			http.Error(w, "empty login or password", http.StatusBadRequest)
			return
		}
		hashPassword := auth.SignPassword(req.Password)
		userIDPtr, err := a.store.GetUser(r.Context(), req.Login, hashPassword)
		if err != nil {
			if errors.Is(err, storage.ErrUserOrPassword) {
				simpleError(w, http.StatusUnauthorized)
				return
			}
			simpleError(w, http.StatusInternalServerError)
			return
		}
		setAuthCookie(*userIDPtr, w)
	}
}

func setAuthCookie(userID models.UserID, w http.ResponseWriter) {
	cookie := auth.CreateAuthCookie(userID)
	http.SetCookie(w, cookie)
}

func (a *App) createOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := usercontext.GetUserID(r.Context())
		if err != nil {
			simpleError(w, http.StatusUnauthorized)
			return
		}

		order, err := io.ReadAll(r.Body)
		orderID := string(order)
		if err != nil || len(orderID) == 0 {
			simpleError(w, http.StatusBadRequest)
			return
		}
		err = checkLuhn(orderID)
		if err != nil {
			simpleError(w, http.StatusUnprocessableEntity)
			return
		}

		err = a.store.CreateOrder(r.Context(), orderID, *userID)
		if err != nil {
			if errors.Is(err, storage.ErrOrderAnotherUser) {
				simpleError(w, http.StatusConflict)
				return
			}
			if errors.Is(err, storage.ErrOrderExists) {
				simpleError(w, http.StatusOK)
				return
			}
			logger.Log.Error("failed CreateOrder", zap.Error(err))
			simpleError(w, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func (a *App) GetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := usercontext.GetUserID(r.Context())
		if err != nil {
			simpleError(w, http.StatusUnauthorized)
			return
		}
		orders, err := a.store.GetUserOrders(r.Context(), *userID)
		if err != nil {
			logger.Log.Error("can not get orders", zap.Error(err), zap.String("user_id", userID.String()))
			simpleError(w, http.StatusInternalServerError)
			return
		}
		if len(orders) == 0 {
			simpleError(w, http.StatusNoContent)
			return
		}

		// порядок важен
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// сериализуем ответ сервера
		// TODO в случае ошибки сериализации клиенту уже отдали статус 200ок
		// а тело будет битым. возможно стоит сначала сериализовать. данных мало поэтому кажется ок
		enc := json.NewEncoder(w)
		if err := enc.Encode(orders); err != nil {
			logger.Log.Error("error encoding response", zap.Error(err))
			return
		}
	}
}

// TODO посмотреть может ли быть балланс дробным
func (a *App) Balance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := usercontext.GetUserID(r.Context())
		if err != nil {
			simpleError(w, http.StatusUnauthorized)
			return
		}
		balance, err := a.store.Balance(r.Context(), *userID)
		if err != nil {
			logger.Log.Error("failed Balance", zap.Error(err))
			simpleError(w, http.StatusInternalServerError)
			return
		}
		// порядок важен
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// сериализуем ответ сервера
		// TODO в случае ошибки сериализации клиенту уже отдали статус 200ок
		// а тело будет битым. возможно стоит сначала сериализовать. данных мало поэтому кажется ок
		enc := json.NewEncoder(w)
		if err := enc.Encode(balance); err != nil {
			logger.Log.Error("error encoding response", zap.Error(err))
			return
		}
	}
}

// TODO вынести авторизацию в middleware
func (a *App) Withdraw() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := usercontext.GetUserID(r.Context())
		if err != nil {
			simpleError(w, http.StatusUnauthorized)
			return
		}

		var req models.WithdrawnRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
			http.Error(w, "bad json", http.StatusUnprocessableEntity)
			return
		}

		err = checkLuhn(req.OrderID)
		if err != nil {
			simpleError(w, http.StatusUnprocessableEntity)
			return
		}

		err = a.store.Withdraw(r.Context(), *userID, req.OrderID, req.Sum)
		if err != nil {
			var code int
			if errors.Is(err, storage.ErrNotEnoughMoney) {
				code = http.StatusPaymentRequired
			} else if errors.Is(err, storage.ErrOrderWithdrawnExists) {
				code = http.StatusUnprocessableEntity
			} else {
				logger.Log.Error("failed Withdraw", zap.Error(err))
				code = http.StatusInternalServerError
			}
			simpleError(w, code)
			return
		}
	}

	// 	POST /api/user/balance/withdraw HTTP/1.1
	// Content-Type: application/json

	// {
	// 	"order": "2377225624",
	//     "sum": 751
	// }
	// ```

	// Здесь `order` — номер заказа, а `sum` — сумма баллов к списанию в счёт оплаты.

	// Возможные коды ответа:

	// - `200` — успешная обработка запроса;
	// - `401` — пользователь не авторизован;
	// - `402` — на счету недостаточно средств;
	// - `422` — неверный номер заказа;
	// - `500` — внутренняя ошибка сервера.

}
