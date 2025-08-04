package models

import (
	"github.com/google/uuid"
)

type RegisterUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Balance struct {
	Current   uint32 `json:"current"`
	Withdrawn uint32 `json:"withdrawn"`
}

type UserID = uuid.UUID
type OrderID = string

type WithdrawnRequest struct {
	OrderID OrderID `json:"order"`
	Sum     uint32  `json:"sum"`
}
