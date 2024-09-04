package models

const (
	OrderStatusPlaced    = "placed"
	OrderStatusPreparing = "preparing"
	OrderStatusReady     = "ready_for_pickup"
	OrderStatusPickedUp  = "picked_up"
	OrderStatusInTransit = "in_transit"
	OrderStatusDelivered = "delivered"
	OrderStatusCancelled = "cancelled"

	PartnerStatusAvailable        = "available"
	PartnerStatusAssigned         = "assigned"
	PartnerStatusEnRoutePickup    = "en_route_to_pickup"
	PartnerStatusWaitingForPickup = "waiting_for_pickup"
	PartnerStatusEnRouteDelivery  = "en_route_to_delivery"
	PartnerStatusDelivering       = "delivering"
	PartnerStatusOffline          = "offline"

	RestaurantStatusOpen   = "open"
	RestaurantStatusClosed = "closed"
)
