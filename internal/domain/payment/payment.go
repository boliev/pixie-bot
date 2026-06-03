package payment

import (
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("payment not found")
	ErrAlreadyPaid = errors.New("payment already processed")
)

type Status string

const (
	StatusPending Status = "pending"
	StatusPaid    Status = "paid"
	StatusFailed  Status = "failed"
)

type ProductCode string

const (
	Product5Credits  ProductCode = "credits_5"
	Product10Credits ProductCode = "credits_10"
	Product25Credits ProductCode = "credits_25"
)

type Product struct {
	Credits     int
	AmountStars int
	Title       string
}

var ProductCatalog = map[ProductCode]Product{
	Product5Credits:  {Credits: 5, AmountStars: 50, Title: "5 кредитов"},
	Product10Credits: {Credits: 10, AmountStars: 90, Title: "10 кредитов"},
	Product25Credits: {Credits: 25, AmountStars: 200, Title: "25 кредитов"},
}

type Payment struct {
	ID                      int64
	UserID                  int64
	TelegramPaymentChargeID string
	ProviderPaymentChargeID string
	ProductCode             ProductCode
	Credits                 int
	AmountStars             int
	Status                  Status
	CreatedAt               time.Time
	UpdatedAt               time.Time
}
