package simulator

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/xitongsys/parquet-go/schema"
	"log"
	"time"
)

// represents time-dependent growth patterns
type GrowthPattern struct {
	BaseRate        float64
	SeasonalFactor  float64
	EventMultiplier float64
	DayOfWeekRates  map[time.Weekday]float64
}

// represents time-based ordering patterns
type OrderPattern struct {
	Type                  string
	BaseProbability       float64
	TimeMultipliers       map[int]float64 // Hour -> multiplier
	WeekdayMultipliers    map[time.Weekday]float64
	WeatherEffects        map[string]float64
	MenuPreferences       map[string]float64 // MenuItem.Type -> preference multiplier
	WeekdayPeakHours      []int
	WeekendPeakHours      []int
	WeekendMultiplier     float64
	FridayNightMultiplier float64
	// Holiday/special event multipliers
	SpecialDates map[string]float64
}

// represents statistical parameters for various metrics
type DistributionParams struct {
	OrderAmount struct {
		Mean float64
		Std  float64
		Min  float64
		Max  float64
	}
	PreparationTime struct {
		Mean float64
		Std  float64
		Min  float64
		Max  float64
	}
	DeliveryTime struct {
		Mean float64
		Std  float64
		Min  float64
		Max  float64
	}
	Ratings struct {
		Food     NormalDistribution
		Delivery NormalDistribution
	}
}

type NormalDistribution struct {
	Mean float64
	Std  float64
	Min  float64
	Max  float64
}

type MenuTimePattern struct {
	ItemType          string
	PeakHours         []int
	SeasonalMonths    []time.Month
	DayPartPreference map[string]float64 // "morning", "afternoon", "evening", "night"
	WeatherPreference map[string]float64
}

type RestaurantPattern struct {
	CuisineType      string
	PeakHours        map[time.Weekday][]int
	PrepTimeRange    TimeRange
	PriceCategory    string
	PopularityFactor float64
	SeasonalItems    map[time.Month][]string
}

type TimeRange struct {
	Min float64
	Max float64
	Std float64
}

type RestaurantCluster struct {
	Type                string
	BaseCapacity        int
	CapacityFlexibility float64
	QualityVariance     float64
	PriceTier           int
	PreparationSpeed    float64
}

type LocalEvent struct {
	Type       string
	Multiplier float64
	StartTime  time.Time
	EndTime    time.Time
}

type WeatherCondition struct {
	Temperature   float64
	Condition     string
	WindSpeed     float64
	Precipitation float64
	Humidity      float64
}

type WeatherState struct {
	Condition     string
	Duration      time.Duration
	StartTime     time.Time
	Intensity     float64 // 0.0 to 1.0
	Temperature   float64
	WindSpeed     float64 // km/h
	Humidity      float64 // 0.0 to 1.0
	Precipitation float64 // mm/hour
}

type WeatherTransition struct {
	Condition         string
	BaseProbability   float64
	SeasonalModifiers map[string]float64
	TimeModifiers     map[string]float64
	MinDuration       time.Duration
	MaxDuration       time.Duration
}

type ReviewPattern struct {
	UserSegment     string
	OrderType       string
	TimeOfDay       string
	BaseProbability float64
	RatingBias      RatingBias
	CommentPatterns []CommentPattern
}

type RatingBias struct {
	FoodBase         float64
	DeliveryBase     float64
	TimeInfluence    float64
	PriceInfluence   float64
	WeatherInfluence float64
}

type CommentPattern struct {
	Sentiment string
	Templates []string
	Triggers  map[string]float64 // condition -> threshold
}

type ReviewMetrics struct {
	totalReviews       int
	averageRating      float64
	ratingDistribution map[float64]int
	reviewFrequency    map[string]int // frequency by date
	commonPhrases      map[string]int
	reviewTimes        []time.Time
}

type RatingWindow struct {
	Duration   time.Duration
	Weight     float64
	MinReviews int
}

type TrendIndicator struct {
	Direction float64 // -1 to 1
	Strength  float64 // 0 to 1
	Period    time.Duration
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

type MarketPosition struct {
	PriceTier      string
	QualityTier    string
	Popularity     float64
	CompetitivePos float64
}

type DemandFactors struct {
	TimeOfDay     float64
	DayOfWeek     float64
	Weather       float64
	Seasonality   float64
	SpecialEvents float64
}

type CompetitiveMetrics struct {
	rating  float64
	price   float64
	volume  float64
	variety float64
}

type UserCreatedEvent struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Email               string          `json:"email"`
	JoinDate            int64           `json:"joinDate"`
	Location            models.Location `json:"location"`
	Preferences         []string        `json:"preferences"`
	DietaryRestrictions []string        `json:"dietRestrictions"`
	OrderFrequency      float64         `json:"orderFrequency"`
	Timestamp           int64           `json:"timestamp" parquet:"name=timestamp,type=INT64"`
}

type RestaurantCreatedEvent struct {
	ID               string          `json:"id"`
	Host             string          `json:"host"`
	Name             string          `json:"name"`
	Currency         int             `json:"currency"`
	Phone            string          `json:"phone"`
	Town             string          `json:"town"`
	SlugName         string          `json:"slugName"`
	WebsiteLogoURL   string          `json:"websiteLogoUrl"`
	Offline          string          `json:"offline"`
	Location         models.Location `json:"location"`
	Cuisines         []string        `json:"cuisines"`
	Rating           float64         `json:"rating"`
	TotalRatings     float64         `json:"totalRatings"`
	PrepTime         float64         `json:"prepTime"`
	MinPrepTime      float64         `json:"minPrepTime"`
	AvgPrepTime      float64         `json:"avgPrepTime"` // Average preparation time in minutes
	PickupEfficiency float64         `json:"pickupEfficiency"`
	MenuItems        []string        `json:"menuItemIds"`
	Capacity         int             `json:"capacity"`
	Timestamp        int64           `json:"timestamp" parquet:"name=timestamp,type=INT64"`
}

type DeliveryPartnerCreatedEvent struct {
	ID              string          `json:"id"`
	Timestamp       int64           `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	Name            string          `json:"name"`
	JoinDate        int64           `json:"joinDate"`
	Rating          float64         `json:"rating"`
	TotalRatings    float64         `json:"totalRatings"`
	Experience      float64         `json:"experienceScore"` // Experience score
	Speed           float64         `json:"speed"`
	AvgSpeed        float64         `json:"avgSpeed"`
	CurrentOrderID  string          `json:"currentOrderID"`
	CurrentLocation models.Location `json:"currentLocation"`
	Status          string          `json:"status"` // "available", "en_route_to_pickup", "en_route_to_delivery"
}

type MenuItemCreatedEvent struct {
	ID                 string   `json:"id"`
	Timestamp          int64    `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	RestaurantID       string   `json:"restaurantID"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Price              float64  `json:"price"`
	PrepTime           float64  `json:"prepTime"` // Preparation time in minutes
	Category           string   `json:"category"`
	Type               string   `json:"type"`       // e.g., "appetizer", "main course", "side dish", "dessert", "drink"
	Popularity         float64  `json:"popularity"` // A score representing item popularity (e.g., 0.0 to 1.0)
	PrepComplexity     float64  `json:"prepComplexity"`
	Ingredients        []string `json:"ingredients"` // List of ingredients
	IsDiscountEligible bool     `json:"isDiscountEligible"`
}

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
	Timestamp         int64           `json:"timestamp" parquet:"name=timestamp,type=INT64"`
	OrderID           string          `json:"orderId" parquet:"name=orderId,type=BYTE_ARRAY,convertedtype=UTF8"`
	Items             []string        `json:"itemIds" parquet:"name=itemIds,type=BYTE_ARRAY,convertedtype=UTF8"`
	TotalAmount       float64         `json:"totalAmount" parquet:"name=totalAmount,type=DOUBLE"`
	OrderPlacedAt     int64           `json:"orderPlacedAt" parquet:"name=orderPlacedAt,type=INT64"`
	CustomerID        string          `json:"customerId"`
	RestaurantID      string          `json:"restaurantId"`
	DeliveryPartnerID string          `json:"deliveryPartnerID"`
	DeliveryCost      float64         `json:"deliveryCost"`
	PaymentMethod     string          `json:"paymentMethod"` // e.g., "card", "cash", "wallet"
	Address           models.Location `json:"deliveryAddress"`
	ReviewGenerated   bool            `json:"reviewGenerated"`
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
