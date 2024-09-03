package models

import "time"

type Simulator struct {
	Config                      Config
	Users                       []User
	DeliveryPartners            []DeliveryPartner
	Reviews                     []Review
	TrafficConditions           []TrafficCondition
	Orders                      []Order
	OrdersByUser                map[string][]Order
	CompletedOrdersByRestaurant map[string][]Order
	Restaurants                 map[string]*Restaurant
	MenuItems                   map[string]*MenuItem
	CurrentTime                 time.Time
}
