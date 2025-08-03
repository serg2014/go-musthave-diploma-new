package models

import (
	"github.com/google/uuid"
)

type RegisterUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type UserID = uuid.UUID
