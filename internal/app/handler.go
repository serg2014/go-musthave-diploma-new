package app

import (
	"net/http"
)

func (a *App) setRoute() {
	r := a.GetRouter()
	r.Post("/api/user/register", a.registerUser())
}

func (a *App) registerUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
	// 	POST /api/user/register HTTP/1.1
	// Content-Type: application/json
	// ...

	// {
	// 	"login": "<login>",
	// 	"password": "<password>"
	// }
	// ```

	// Возможные коды ответа:

	// - `200` — пользователь успешно зарегистрирован и аутентифицирован;
	// - `400` — неверный формат запроса;
	// - `409` — логин уже занят;
	// - `500` — внутренняя ошибка сервера.
}
