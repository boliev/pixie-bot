package credits

import "time"

type TransactionType string

const (
	TypeBonus    TransactionType = "bonus"
	TypeDebit    TransactionType = "debit"
	TypeRefund   TransactionType = "refund"
	TypePurchase TransactionType = "purchase"
)

type CreditTransaction struct {
	ID         int64
	UserID     int64
	Type       TransactionType
	Amount     int
	Reason     string
	ExternalID string
	CreatedAt  time.Time
}
