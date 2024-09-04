package models

const (
	OrderStatusPlaced    = "placed"
	OrderStatusPreparing = "preparing"
	OrderStatusReady     = "ready"
	OrderStatusPickedUp  = "picked_up"
	OrderStatusInTransit = "in_transit"
	OrderStatusDelivered = "delivered"
	OrderStatusCancelled = "cancelled"

	PartnerStatusAvailable     = "available"
	PartnerStatusAssigned      = "assigned"
	PartnerStatusEnRoutePickup = "en_route_to_pickup"
	PartnerStatusDelivering    = "delivering"
	PartnerStatusOffline       = "offline"

	RestaurantStatusOpen   = "open"
	RestaurantStatusClosed = "closed"
)
