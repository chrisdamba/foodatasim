package models

import "time"

type Review struct {
	ID                string    `json:"id"`
	OrderID           string    `json:"order_id"`
	CustomerID        string    `json:"customer_id"`
	RestaurantID      string    `json:"restaurant_id"`
	DeliveryPartnerID string    `json:"delivery_partner_id"`
	FoodRating        int       `json:"food_rating"`
	DeliveryRating    int       `json:"delivery_rating"`
	OverallRating     int       `json:"overall_rating"`
	Comment           string    `json:"comment"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	IsIgnored         bool      `json:"is_ignored"`
}
