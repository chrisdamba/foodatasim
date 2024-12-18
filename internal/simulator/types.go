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
	ID                string         `json:"id" parquet:"name=id,type=BYTE_ARRAY,convertedtype=UTF8"`
	CustomerID        string         `json:"customerId,omitempty" parquet:"name=customerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	RestaurantID      string         `json:"restaurantId,omitempty" parquet:"name=restaurantId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID string         `json:"deliveryPartnerId,omitempty" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	ItemIDs           []string       `json:"itemIds" parquet:"name=itemIds,type=BYTE_ARRAY,convertedtype=UTF8"`
	TotalAmount       float64        `json:"totalAmount" parquet:"name=totalAmount,type=DOUBLE"`
	DeliveryCost      float64        `json:"deliveryCost" parquet:"name=deliveryCost,type=DOUBLE"`
	PaymentMethod     string         `json:"paymentMethod"  parquet:"name=paymentMethod,type=BYTE_ARRAY,convertedtype=UTF8"`
	OrderPlacedAt     time.Time      `json:"orderPlacedAt" parquet:"name=orderPlacedAt,type=INT64"`
	DeliveryAddress   models.Address `json:"deliveryAddress" parquet:"name=newLocation,type=STRUCT"`
}

// OrderPreparationEvent represents an order being prepared
type OrderPreparationEvent struct {
	BaseEvent
	OrderID         string         `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status          string         `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PrepStartTime   time.Time      `json:"prepStartTime" parquet:"name=prepStartTime,type=INT64"`
	DeliveryAddress models.Address `json:"deliveryAddress" parquet:"name=newLocation,type=STRUCT"`
}

// OrderReadyEvent represents an order being ready for pickup
type OrderReadyEvent struct {
	BaseEvent
	OrderID         string         `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status          string         `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PickupTime      time.Time      `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
	DeliveryAddress models.Address `json:"deliveryAddress" parquet:"name=newLocation,type=STRUCT"`
}

// DeliveryPartnerAssignmentEvent represents a delivery partner being assigned to an order
type DeliveryPartnerAssignmentEvent struct {
	BaseEvent
	OrderID             string    `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status              string    `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedPickupTime time.Time `json:"estimatedPickupTime" parquet:"name=estimatedPickupTime,type=INT64"`
}

// OrderPickupEvent represents an order being picked up by a delivery partner
type OrderPickupEvent struct {
	BaseEvent
	OrderID               string    `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string    `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	PickupTime            time.Time `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
	EstimatedDeliveryTime time.Time `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
}

// PartnerLocationUpdateEvent represents an update to a delivery partner's location
type PartnerLocationUpdateEvent struct {
	Timestamp         time.Time       `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	EventType         string          `json:"eventType" parquet:"name=eventType,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID string          `json:"deliveryPartnerId" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	OrderID           string          `json:"orderId,omitempty" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	NewLocation       models.Location `json:"newLocation" parquet:"name=newLocation,type=STRUCT"`
	CurrentOrder      string          `json:"currentOrder,omitempty" parquet:"name=currentOrder,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	Status            string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8"`
	UpdateTime        time.Time       `json:"updateTime" parquet:"name=updateTime,type=INT64"`
	Speed             float64         `json:"speed,omitempty" parquet:"name=speed,type=DOUBLE,repetitiontype=OPTIONAL"`
}

// OrderInTransitEvent represents an order being in transit
type OrderInTransitEvent struct {
	BaseEvent
	OrderID               string          `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID     string          `json:"deliveryPartnerId" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	CurrentLocation       models.Location `json:"currentLocation" parquet:"name=currentLocation,type=STRUCT"`
	EstimatedDeliveryTime time.Time       `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	PickupTime            time.Time       `json:"pickupTime" parquet:"name=pickupTime,type=INT64"`
	Status                string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
}

// DeliveryStatusCheckEvent represents a check on the delivery status
type DeliveryStatusCheckEvent struct {
	BaseEvent
	OrderID               string          `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string          `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedDeliveryTime time.Time       `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	CurrentLocation       models.Location `json:"currentLocation" parquet:"name=currentLocation,type=STRUCT"`
	NextCheckTime         time.Time       `json:"nextCheckTime" parquet:"name=nextCheckTime,type=INT64"`
}

type DeliveryPerformanceEvent struct {
	ID                string           `json:"id"`
	DeliveryPartnerID string           `json:"delivery_partner_id"`
	OrderID           string           `json:"order_id"`
	Timestamp         time.Time        `json:"timestamp"`
	EventType         string           `json:"event_type"`
	CurrentLocation   models.Location  `json:"current_location"`
	NewLocation       *models.Location `json:"new_location,omitempty"`
	EstimatedArrival  time.Time        `json:"estimated_arrival"`
	ActualArrival     time.Time        `json:"actual_arrival"`
	Speed             float64          `json:"speed"`
	DistanceCovered   float64          `json:"distance_covered"`
	Status            string           `json:"status"`
	UpdateTime        time.Time        `json:"update_time"`
	CurrentOrder      string           `json:"current_order"`
}

type RestaurantPerformanceEvent struct {
	ID              string    `json:"id"`
	RestaurantID    string    `json:"restaurant_id"`
	Timestamp       time.Time `json:"timestamp"`
	EventType       string    `json:"event_type"`
	CurrentCapacity int       `json:"current_capacity"`
	Capacity        int       `json:"capacity"`
	OrdersInQueue   int       `json:"orders_in_queue"`
	PrepTime        float64   `json:"prep_time"`
	AvgPrepTime     float64   `json:"avg_prep_time"`
	CurrentLoad     float64   `json:"current_load"`
	EfficiencyScore float64   `json:"efficiency_score"`
	CreatedAt       time.Time `json:"created_at"`
}

// OrderDeliveryEvent represents an order being delivered
type OrderDeliveryEvent struct {
	BaseEvent
	OrderID               string    `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status                string    `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	EstimatedDeliveryTime time.Time `json:"estimatedDeliveryTime" parquet:"name=estimatedDeliveryTime,type=INT64"`
	ActualDeliveryTime    time.Time `json:"actualDeliveryTime" parquet:"name=actualDeliveryTime,type=INT64"`
}

// OrderCancellationEvent represents an order being cancelled
type OrderCancellationEvent struct {
	BaseEvent
	OrderID          string    `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Status           string    `json:"status" parquet:"name=status,type=BYTE_ARRAY,convertedtype=UTF8"`
	CancellationTime time.Time `json:"cancellationTime" parquet:"name=cancellationTime,type=INT64"`
}

// UserBehaviourUpdateEvent represents an update to a user's behaviour
type UserBehaviourUpdateEvent struct {
	Timestamp      time.Time `json:"timestamp" parquet:"name=timestamp,type=INT64,repetitiontype=OPTIONAL"`
	EventType      *string   `json:"eventType" parquet:"name=eventType,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	UserID         *string   `json:"userId" parquet:"name=userId,type=BYTE_ARRAY,convertedtype=BYTE_ARRAY,convertedtype=UTF8,repetitiontype=OPTIONAL"`
	OrderFrequency *float64  `json:"orderFrequency" parquet:"name=orderFrequency,type=DOUBLE,repetitiontype=OPTIONAL"`
	LastOrderTime  time.Time `json:"lastOrderTime,omitempty" parquet:"name=lastOrderTime,type=INT64,repetitiontype=OPTIONAL"`
}

// RestaurantStatusUpdateEvent represents an update to a restaurant's status
type RestaurantStatusUpdateEvent struct {
	BaseEvent
	Capacity        int32   `json:"capacity" parquet:"name=capacity,type=INT32"`
	CurrentCapacity int32   `json:"current_capacity" parquet:"name=current_capacity,type=INT32"`
	OrdersInQueue   int32   `json:"orders_in_queue" parquet:"name=orders_in_queue,type=INT32"`
	PrepTime        float64 `json:"prep_time" parquet:"name=prep_time,type=DOUBLE"`
}

// ReviewEvent represents a review being generated
type ReviewEvent struct {
	BaseEvent
	ReviewID          string    `json:"reviewId" parquet:"name=reviewId,type=BYTE_ARRAY,convertedtype=UTF8"`
	OrderID           string    `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	CustomerID        string    `json:"customerId" parquet:"name=customerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	DeliveryPartnerID string    `json:"deliveryPartnerId" parquet:"name=deliveryPartnerId,type=BYTE_ARRAY,convertedtype=UTF8"`
	FoodRating        float64   `json:"foodRating" parquet:"name=foodRating,type=DOUBLE"`
	DeliveryRating    float64   `json:"deliveryRating" parquet:"name=deliveryRating,type=DOUBLE"`
	OverallRating     float64   `json:"overallRating" parquet:"name=overallRating,type=DOUBLE"`
	Comment           string    `json:"comment" parquet:"name=comment,type=BYTE_ARRAY,convertedtype=UTF8"`
	CreatedAt         time.Time `json:"createdAt" parquet:"name=createdAt,type=INT64"`
	OrderTotal        float64   `json:"orderTotal" parquet:"name=orderTotal,type=DOUBLE"`
	DeliveryTime      int64     `json:"deliveryTime" parquet:"name=deliveryTime,type=INT64"`
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
