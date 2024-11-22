package models

import "time"

type Restaurant struct {
	ID                string                      `json:"id"`
	Host              string                      `json:"host"`
	Name              string                      `json:"name"`
	Currency          int                         `json:"currency"`
	Phone             string                      `json:"phone"`
	Town              string                      `json:"town"`
	SlugName          string                      `json:"slug_name"`
	WebsiteLogoURL    string                      `json:"website_logo_url"`
	Offline           string                      `json:"offline"`
	Location          Location                    `json:"location"`
	Cuisines          []string                    `json:"cuisines"`
	Rating            float64                     `json:"rating"`
	TotalRatings      float64                     `json:"total_ratings"`
	PrepTime          float64                     `json:"prep_time"`
	MinPrepTime       float64                     `json:"min_prep_time"`
	AvgPrepTime       float64                     `json:"avg_prep_time"` // Average preparation time in minutes
	PickupEfficiency  float64                     `json:"pickup_efficiency"`
	MenuItems         []string                    `json:"menu_item_ids,omitempty"`
	CurrentOrders     []Order                     `json:"current_orders"`
	Capacity          int                         `json:"capacity"`
	PriceTier         string                      `json:"price_tier"` // "budget", "standard", "premium"
	ReputationMetrics ReputationMetrics           `json:"reputation_metrics"`
	ReputationHistory []ReputationMetrics         `json:"reputation_history"`
	DemandPatterns    map[int]float64             `json:"demand_patterns"` // Hour -> historical demand
	MarketPosition    MarketPosition              `json:"market_position"`
	PopularityMetrics RestaurantPopularityMetrics `json:"popularity_metrics"`
}

type RestaurantPopularityMetrics struct {
	BasePopularity    float64
	TrendFactor       float64
	TimeBasedDemand   map[int]float64    // Hour -> demand multiplier
	CustomerSegments  map[string]float64 // Segment -> preference multiplier
	PriceAppeal       float64
	QualityAppeal     float64
	ConsistencyAppeal float64
}

type ReputationMetrics struct {
	BaseRating        float64   `json:"base_rating"`
	ConsistencyScore  float64   `json:"consistency_score"`
	TrendScore        float64   `json:"trend_score"`
	ReliabilityScore  float64   `json:"reliability_score"`
	ResponseScore     float64   `json:"response_score"`
	PriceQualityScore float64   `json:"price_quality_score"`
	LastUpdate        time.Time `json:"last_update"`
}

type MarketPosition struct {
	PriceTier      string  `json:"price_tier"`
	QualityTier    string  `json:"quality_tier"`
	Popularity     float64 `json:"popularity"`
	CompetitivePos float64 `json:"competitive_position"`
}

type ItemPopularity struct {
	ID    string
	Count int
}

type RestaurantPerformanceCache struct {
	HourlyOrderCounts map[int]int
	PeakHours         []int
	PopularItems      []ItemPopularity
	RecentMetrics     OrderMetrics
	LastUpdate        time.Time
}
