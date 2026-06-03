package user

import (
	"errors"
	"time"
)

const InitialCredits = 3

var (
	ErrNotFound            = errors.New("user not found")
	ErrAlreadyExists       = errors.New("user already exists")
	ErrInsufficientCredits = errors.New("insufficient credits")
)

type User struct {
	ID             int64
	TelegramUserID int64
	Username       string
	FirstName      string
	LastName       string
	Credits        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(telegramUserID int64, username, firstName, lastName string) *User {
	now := time.Now()

	return &User{
		TelegramUserID: telegramUserID,
		Username:       username,
		FirstName:      firstName,
		LastName:       lastName,
		Credits:        0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
