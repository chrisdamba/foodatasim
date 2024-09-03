package models

import "time"

type TrafficCondition struct {
	Time     time.Time `json:"time"`
	Location Location  `json:"location"`
	Density  float64   `json:"density"` // Traffic density score
}
