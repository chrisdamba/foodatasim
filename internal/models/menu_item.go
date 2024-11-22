package models

import "time"

type MenuItem struct {
	ID                 string          `json:"id"`
	RestaurantID       string          `json:"restaurant_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Price              float64         `json:"price"`
	PrepTime           float64         `json:"prep_time"` // Preparation time in minutes
	Category           string          `json:"category"`
	Type               string          `json:"type"`       // e.g., "appetizer", "main course", "side dish", "dessert", "drink"
	Popularity         float64         `json:"popularity"` // A score representing item popularity (e.g., 0.0 to 1.0)
	PrepComplexity     float64         `json:"prep_complexity"`
	Ingredients        []string        `json:"ingredients"` // List of ingredients
	IsDiscountEligible bool            `json:"is_discount_eligible"`
	BasePrice          float64         `json:"base_price"`
	DynamicPricing     bool            `json:"dynamic_pricing"`
	PriceHistory       []PricePoint    `json:"price_history"`
	SeasonalFactor     float64         `json:"seasonal_factor"`
	TimeBasedDemand    map[int]float64 `json:"time_based_demand"` // Hour -> demand multiplier
}

type PricePoint struct {
	Price     float64            `json:"price"`
	Timestamp time.Time          `json:"timestamp"`
	Factors   map[string]float64 `json:"factors"` // Factor name -> multiplier
}
