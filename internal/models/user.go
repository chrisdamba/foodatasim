package models

import (
	"time"
)

type User struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	JoinDate            time.Time `json:"join_date"`
	Location            Location  `json:"location"`
	Preferences         []string  `json:"preferences"`
	DietaryRestrictions []string  `json:"diet_restrictions"`
	OrderFrequency      float64   `json:"order_frequency"`
	LastOrderTime       time.Time `json:"last_order_time"`
}

type UserBehaviourUpdate struct {
	UserID         string
	OrderFrequency float64
}
