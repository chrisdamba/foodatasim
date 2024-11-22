package models

import (
	"time"
)

type OrderHistory struct {
	RecentOrders    []string               `json:"recent_orders"`
	FavoriteItems   map[string]int         `json:"favorite_items"`  // MenuItemID -> order count
	PreferredTimes  map[time.Weekday][]int `json:"preferred_times"` // preferred ordering times
	LastOrderTime   time.Time              `json:"last_order_time"`
	PurchasePattern map[time.Weekday][]int `json:"purchase_pattern"` // actual historical ordering pattern (hours) per day
}

type User struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Email               string                 `json:"email"`
	JoinDate            time.Time              `json:"join_date"`
	Location            Location               `json:"location"`
	Preferences         []string               `json:"preferences"`
	DietaryRestrictions []string               `json:"diet_restrictions"`
	OrderFrequency      float64                `json:"order_frequency"`
	LastOrderTime       time.Time              `json:"last_order_time"`
	Segment             string                 `json:"segment"` // "frequent", "regular", "occasional"
	BehaviorProfile     UserBehaviourProfile   `json:"behaviour_profile"`
	OrderHistory        *OrderHistory          `json:"order_history"`
	PurchasePatterns    map[time.Weekday][]int `json:"purchase_patterns"` // historical ordering hours by day
	LifetimeOrders      int                    `json:"lifetime_orders"`
	LifetimeSpend       float64                `json:"lifetime_spend"`
}

type UserBehaviourUpdate struct {
	UserID         string
	OrderFrequency float64
}

type UserBehaviourProfile struct {
	OrderTimingPreference map[time.Weekday][]int `json:"order_timing_preference"`
	PriceThresholds       struct {
		Min    float64 `json:"min"`
		Max    float64 `json:"max"`
		Target float64 `json:"target"`
	} `json:"price_thresholds"`
	OrderSizePreference struct {
		Min     int `json:"min"`
		Max     int `json:"max"`
		Typical int `json:"typical"`
	} `json:"order_size_preference"`
	CuisineWeights     map[string]float64 `json:"cuisine_weights"`
	PaymentPreferences map[string]float64 `json:"payment_preferences"`
	LocationPreference struct {
		MaxDistance    float64  `json:"max_distance"`
		PreferredAreas []string `json:"preferred_areas"`
	} `json:"location_preference"`
}

type CustomerSegment struct {
	Name           string
	Ratio          float64
	OrdersPerMonth float64
	AvgSpend       float64
	PeakHourBias   float64
	// Preferences for cuisine types, weighted
	CuisinePreferences map[string]float64
}

var DefaultCustomerSegments = map[string]CustomerSegment{
	"frequent": {
		Name:           "frequent",
		Ratio:          0.2,
		OrdersPerMonth: 12,
		AvgSpend:       50.0,
		PeakHourBias:   1.2,
		CuisinePreferences: map[string]float64{
			"fast_food": 0.3,
			"healthy":   0.2,
			"premium":   0.5,
		},
	},
	"regular": {
		Name:           "regular",
		Ratio:          0.5,
		OrdersPerMonth: 6,
		AvgSpend:       35.0,
		PeakHourBias:   1.0,
		CuisinePreferences: map[string]float64{
			"fast_food": 0.4,
			"healthy":   0.3,
			"premium":   0.3,
		},
	},
	"occasional": {
		Name:           "occasional",
		Ratio:          0.3,
		OrdersPerMonth: 2,
		AvgSpend:       20.0,
		PeakHourBias:   0.8,
		CuisinePreferences: map[string]float64{
			"fast_food": 0.5,
			"healthy":   0.3,
			"premium":   0.2,
		},
	},
}
