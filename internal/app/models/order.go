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

type ProcessingOrderItem struct {
	OrderID OrderID
	UserID  UserID
}

type ProcessingOrders []ProcessingOrderItem

type AccrualOrderStatus string

const (
	AccrualOrderRegistered AccrualOrderStatus = "REGISTERED"
	AccrualOrderProcessing AccrualOrderStatus = "PROCESSING"
	AccrualOrderInvalid    AccrualOrderStatus = "INVALID"
	AccrualOrderProcessed  AccrualOrderStatus = "PROCESSED"
)

var AccrualOrderTerminateStatus []AccrualOrderStatus = []AccrualOrderStatus{
	AccrualOrderInvalid,
	AccrualOrderProcessed,
}

type AccrualOrderItem struct {
	OrderID OrderID            `json:"order"`
	UserID  UserID             `json:"-"`
	Status  AccrualOrderStatus `json:"status"`
	Accrual *uint64            `json:"accrual,omitempty"`
	Error   error              `json:"-"`
}
