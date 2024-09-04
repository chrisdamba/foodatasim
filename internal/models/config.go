package models

import "time"

type Config struct {
	Seed               int       `json:"seed"`
	StartDate          time.Time `json:"start_date"`
	EndDate            time.Time `json:"end_date"`
	InitialUsers       int       `json:"initial_users"`
	InitialRestaurants int       `json:"initial_restaurants"`
	InitialPartners    int       `json:"initial_partners"`
	UserGrowthRate     float64   `json:"user_growth_rate"`
	PartnerGrowthRate  float64   `json:"partner_growth_rate"`
	OrderFrequency     float64   `json:"order_frequency"`
	PeakHourFactor     float64   `json:"peak_hour_factor"`
	WeekendFactor      float64   `json:"weekend_factor"`
	TrafficVariability float64   `json:"traffic_variability"`

	// Additional fields
	CityName              string  `json:"city_name"`
	DefaultCurrency       int     `json:"default_currency"`
	MinPrepTime           int     `json:"min_prep_time"`
	MaxPrepTime           int     `json:"max_prep_time"`
	MinRating             float64 `json:"min_rating"`
	MaxRating             float64 `json:"max_rating"`
	MaxInitialRatings     float64 `json:"max_initial_ratings"`
	MinEfficiency         float64 `json:"min_efficiency"`
	MaxEfficiency         float64 `json:"max_efficiency"`
	MinCapacity           int     `json:"min_capacity"`
	MaxCapacity           int     `json:"max_capacity"`
	TaxRate               float64 `json:"tax_rate"`
	ServiceFeePercentage  float64 `json:"service_fee_percentage"`
	DiscountPercentage    float64 `json:"discount_percentage"`
	MinOrderForDiscount   float64 `json:"min_order_for_discount"`
	MaxDiscountAmount     float64 `json:"max_discount_amount"`
	BaseDeliveryFee       float64 `json:"base_delivery_fee"`
	FreeDeliveryThreshold float64 `json:"free_delivery_threshold"`
	SmallOrderThreshold   float64 `json:"small_order_threshold"`
	SmallOrderFee         float64 `json:"small_order_fee"`
	RestaurantRatingAlpha float64 `json:"restaurant_rating_alpha"`
	PartnerRatingAlpha    float64 `json:"partner_rating_alpha"`

	NearLocationThreshold float64 `json:"near_location_threshold"`
	CityLat               float64 `json:"city_latitude"`
	CityLon               float64 `json:"city_longitude"`
	UrbanRadius           float64 `json:"urban_radius"`
	HotspotRadius         float64 `json:"hotspot_radius"`
	PartnerMoveSpeed      float64 `json:"partner_move_speed"`   // km per time unit
	LocationPrecision     float64 `json:"location_precision"`   // For isAtLocation
	UserBehaviorWindow    int     `json:"user_behavior_window"` // Number of orders to consider for adjusting frequency
	RestaurantLoadFactor  float64 `json:"restaurant_load_factor"`
	EfficiencyAdjustRate  float64 `json:"efficiency_adjust_rate"`
}
