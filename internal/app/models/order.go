package models

import "time"

type OrderStatus string

const (
	OrderNew        OrderStatus = "NEW"
	OrderProcessing OrderStatus = "PROCESSING"
	OrderInvalid    OrderStatus = "INVALID"
	OrderProcessed  OrderStatus = "PROCESSED"
)

type OrderID = string

type OrderItem struct {
	OrderID    OrderID     `json:"number"`
	Status     OrderStatus `json:"status"`
	Accrual    *uint64     `json:"accrual,omitempty"`
	UploadTime time.Time   `json:"uploaded_at"`
}
type Orders []OrderItem
