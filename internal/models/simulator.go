package models

import "time"

type Simulator struct {
	Config            Config
	Users             []User
	Restaurants       []Restaurant
	MenuItems         []MenuItem
	DeliveryPartners  []DeliveryPartner
	Orders            []Order
	TrafficConditions []TrafficCondition
	CurrentTime       time.Time
}
