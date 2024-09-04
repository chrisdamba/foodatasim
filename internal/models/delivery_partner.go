package models

import "time"

type DeliveryPartner struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	JoinDate        time.Time `json:"join_date"`
	Rating          float64   `json:"rating"`
	TotalRatings    float64   `json:"total_ratings"`
	Experience      float64   `json:"experience"` // Experience score
	AvgSpeed        float64   `json:"avg_speed"`
	CurrentOrderID  string    `json:"current_order_id"`
	CurrentLocation Location  `json:"current_location"`
	Status          string    `json:"status"` // "available", "en_route_to_pickup", "en_route_to_delivery"
}
