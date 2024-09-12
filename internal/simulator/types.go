package simulator

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/xitongsys/parquet-go/schema"
	"log"
	"time"
)

// BaseEvent is the common structure for all events
type BaseEvent struct {
	Timestamp    int64  `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	EventType    string `json:"eventType" parquet:"name=eventType,type=BYTE_ARRAY,convertedtype=UTF8"`
	UserID       string `json:"userId,omitempty" parquet:"name=userId,type=BYTE_ARRAY,convertedtype=UTF8"`
	RestaurantID string `json:"restaurantId,omitempty" parquet:"name=restaurantId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryID   string `json:"deliveryPartnerId,omitempty" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
}

// OrderPlacedEvent represents an order being placed
type OrderPlacedEvent struct {
	BaseEvent
	OrderID       string  `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Items         string  `json:"itemIds" parquet:"name=itemIds,type=BYTE_ARRAY,convertedtype=UTF8"`
	TotalAmount   float64 `json:"totalAmount" parquet:"name=totalAmount,type=DOUBLE"`
	Status        string  `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	OrderPlacedAt int64   `json:"orderPlacedAt" parquet:"name=orderPlacedAt,type=INT64"`
}

// OrderPreparationEvent represents an order being prepared
type OrderPreparationEvent struct {
	BaseEvent
	OrderID       string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status        string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PrepStartTime int64  `json:"prepStartTime" parquet:"name=prepStartTime,type=INT64"`
}

// OrderReadyEvent represents an order being ready for pickup
type OrderReadyEvent struct {
	BaseEvent
	OrderID    string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status     string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PickupTime int64  `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
}

// DeliveryPartnerAssignmentEvent represents a delivery partner being assigned to an order
type DeliveryPartnerAssignmentEvent struct {
	BaseEvent
	OrderID             string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status              string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedPickupTime int64  `json:"estimatedPickupTime" parquet:"name=estimatedPickupTime,type=INT64"`
}

// OrderPickupEvent represents an order being picked up by a delivery partner
type OrderPickupEvent struct {
	BaseEvent
	OrderID               string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PickupTime            int64  `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
	EstimatedDeliveryTime int64  `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
}

// PartnerLocationUpdateEvent represents an update to a delivery partner's location
type PartnerLocationUpdateEvent struct {
	Timestamp    int64           `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	EventType    string          `json:"eventType" parquet:"name=eventType,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	PartnerID    string          `json:"partnerId" parquet:"name=partnerId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	OrderID      string          `json:"orderId,omitempty" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	NewLocation  models.Location `json:"newLocation" parquet:"name=newLocation,type=STRUCT"`
	CurrentOrder string          `json:"currentOrder,omitempty" parquet:"name=currentOrder,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	Status       string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	UpdateTime   int64           `json:"updateTime" parquet:"name=updateTime,type=INT64"`
	Speed        float64         `json:"speed,omitempty" parquet:"name=speed,type=DOUBLE,repetitiontype=OPTIONAL"`
}

// OrderInTransitEvent represents an order being in transit
type OrderInTransitEvent struct {
	BaseEvent
	OrderID               string          `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID     string          `json:"deliveryPartnerId" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	CustomerID            string          `json:"customerId" parquet:"name=customerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	CurrentLocation       models.Location `json:"currentLocation" parquet:"name=currentLocation,type=STRUCT"`
	EstimatedDeliveryTime int64           `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	PickupTime            int64           `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
	Status                string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
}

// DeliveryStatusCheckEvent represents a check on the delivery status
type DeliveryStatusCheckEvent struct {
	BaseEvent
	OrderID               string          `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedDeliveryTime int64           `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	CurrentLocation       models.Location `json:"currentLocation" parquet:"name=currentLocation,type=STRUCT"`
	NextCheckTime         int64           `json:"nextCheckTime" parquet:"name=nextCheckTime,type=INT64"`
}

// OrderDeliveryEvent represents an order being delivered
type OrderDeliveryEvent struct {
	BaseEvent
	OrderID               string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedDeliveryTime int64  `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	ActualDeliveryTime    int64  `json:"actualDeliveryTime" parquet:"name=actualDeliveryTime,type=INT64"`
}

// OrderCancellationEvent represents an order being cancelled
type OrderCancellationEvent struct {
	BaseEvent
	OrderID          string `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status           string `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	CancellationTime int64  `json:"cancellationTime" parquet:"name=cancellationTime,type=INT64"`
}

// UserBehaviourUpdateEvent represents an update to a user's behaviour
type UserBehaviourUpdateEvent struct {
	Timestamp      *int64   `json:"timestamp" parquet:"name=timestamp,type=INT64,repetitiontype=OPTIONAL"`
	EventType      *string  `json:"eventType" parquet:"name=eventType,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	UserID         *string  `json:"userId" parquet:"name=userId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	OrderFrequency *float64 `json:"orderFrequency" parquet:"name=orderFrequency,type=DOUBLE,repetitiontype=OPTIONAL"`
	LastOrderTime  *int64   `json:"lastOrderTime,omitempty" parquet:"name=lastOrderTime,type=INT64,repetitiontype=OPTIONAL"`
}

// RestaurantStatusUpdateEvent represents an update to a restaurant's status
type RestaurantStatusUpdateEvent struct {
	BaseEvent
	Capacity *int     `json:"capacity" parquet:"name=capacity,type=INT32,repetitiontype=OPTIONAL"`
	PrepTime *float64 `json:"prepTime" parquet:"name=prepTime,type=DOUBLE,repetitiontype=OPTIONAL"`
}

// ReviewEvent represents a review being generated
type ReviewEvent struct {
	BaseEvent
	ReviewID          string  `json:"reviewId" parquet:"name=reviewId,type=BYTE_ARRAY,convertedtype=UTF8"`
	OrderID           string  `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	CustomerID        string  `json:"customerId" parquet:"name=customerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID string  `json:"deliveryPartnerId" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	FoodRating        float64 `json:"foodRating" parquet:"name=foodRating,type=DOUBLE"`
	DeliveryRating    float64 `json:"deliveryRating" parquet:"name=deliveryRating,type=DOUBLE"`
	OverallRating     float64 `json:"overallRating" parquet:"name=overallRating,type=DOUBLE"`
	Comment           string  `json:"comment" parquet:"name=comment,type=BYTE_ARRAY,convertedtype=UTF8"`
	CreatedAt         int64   `json:"createdAt" parquet:"name=createdAt,type=INT64"`
	OrderTotal        float64 `json:"orderTotal" parquet:"name=orderTotal,type=DOUBLE"`
	DeliveryTime      int64   `json:"deliveryTime" parquet:"name=deliveryTime,type=INT64"`
}

func GetSchema(eventType string) (*schema.SchemaHandler, error) {
	var sh *schema.SchemaHandler
	var err error

	switch eventType {
	case "order_placed_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderPlacedEvent))
	case "order_preparation_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderPreparationEvent))
	case "order_ready_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderReadyEvent))
	case "delivery_partner_assignment_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(DeliveryPartnerAssignmentEvent))
	case "order_pickup_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderPickupEvent))
	case "partner_location_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(PartnerLocationUpdateEvent))
	case "order_in_transit_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderInTransitEvent))
	case "delivery_status_check_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(DeliveryStatusCheckEvent))
	case "order_delivery_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderDeliveryEvent))
	case "order_cancellation_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(OrderCancellationEvent))
	case "user_behaviour_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(UserBehaviourUpdateEvent))
	case "restaurant_status_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(RestaurantStatusUpdateEvent))
	case "review_events":
		sh, err = schema.NewSchemaHandlerFromStruct(new(ReviewEvent))
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	if err != nil {
		log.Printf("Error creating schema for %s: %v", eventType, err)
		// log the actual schema definition
		return nil, fmt.Errorf("error creating schema for %s: %w", eventType, err)
	}

	return sh, nil
}

func NewBaseEvent(eventType string, timestamp time.Time) BaseEvent {
	return BaseEvent{
		Timestamp: timestamp.Unix(),
		EventType: eventType,
	}
}
