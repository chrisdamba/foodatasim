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
}
