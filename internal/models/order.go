package models

import "time"

type Order struct {
	ID                    string    `json:"id"`
	CustomerID            string    `json:"customer_id"`
	RestaurantID          string    `json:"restaurant_id"`
	DeliveryPartnerID     string    `json:"delivery_partner_id"`
	Items                 []string  `json:"item_ids"` // List of MenuItem IDs
	TotalAmount           float64   `json:"total_amount"`
	DeliveryCost          float64   `json:"delivery_cost"`
	OrderPlacedAt         time.Time `json:"order_placed_at"`
	PrepStartTime         time.Time `json:"prep_start_time"`
	EstimatedPickupTime   time.Time `json:"estimated_pickup_time"`
	EstimatedDeliveryTime time.Time `json:"estimated_delivery_time"`
	PickupTime            time.Time `json:"pickup_time"`
	InTransitTime         time.Time `json:"in_transit_time"`
	ActualDeliveryTime    time.Time `json:"actual_delivery_time"`
	Status                string    `json:"status"`         // e.g., "placed", "preparing", "in_transit", "delivered", "cancelled"
	PaymentMethod         string    `json:"payment_method"` // e.g., "card", "cash", "wallet"
	Address               Address   `json:"delivery_address"`
	ReviewGenerated       bool      `json:"review_generated"`
}

type OrderMetrics struct {
	TotalOrders     int
	TotalRevenue    float64
	AvgOrderValue   float64
	AvgPrepTime     float64
	AvgDeliveryTime float64
	PeakHours       map[int]int    // Hour -> Order Count
	PopularItems    map[string]int // ItemID -> Order Count
	CompletionRate  float64
	LateDeliveries  int
	EarlyDeliveries int
}
