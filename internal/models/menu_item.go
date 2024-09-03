package models

type MenuItem struct {
	ID           string  `json:"id"`
	RestaurantID string  `json:"restaurant_id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	PrepTime     float64 `json:"prep_time"` // Preparation time in minutes
	Category     string  `json:"category"`
}
