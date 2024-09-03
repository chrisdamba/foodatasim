package models

type MenuItem struct {
	ID                 string   `json:"id"`
	RestaurantID       string   `json:"restaurant_id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Price              float64  `json:"price"`
	PrepTime           float64  `json:"prep_time"` // Preparation time in minutes
	Category           string   `json:"category"`
	Type               string   `json:"type"`       // e.g., "appetizer", "main course", "side dish", "dessert", "drink"
	Popularity         float64  `json:"popularity"` // A score representing item popularity (e.g., 0.0 to 1.0)
	PrepComplexity     float64  `json:"prep_complexity"`
	Ingredients        []string `json:"ingredients"` // List of ingredients
	IsDiscountEligible bool     `json:"is_discount_eligible"`
}
