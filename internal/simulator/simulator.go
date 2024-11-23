package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/factories"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/chrisdamba/foodatasim/internal/repositories"
	"github.com/chrisdamba/foodatasim/internal/repositories/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

type Simulator struct {
	Config                      *models.Config
	UserRepo                    repositories.UserRepository
	RestaurantRepo              repositories.RestaurantRepository
	MenuItemRepo                repositories.MenuItemRepository
	DeliveryPartnerRepo         repositories.DeliveryPartnerRepository
	Users                       []*models.User
	DeliveryPartners            []*models.DeliveryPartner
	TrafficConditions           []models.TrafficCondition
	Orders                      []models.Order
	Reviews                     []models.Review
	OrdersByUser                map[string][]models.Order
	CompletedOrdersByRestaurant map[string][]models.Order
	Restaurants                 map[string]*models.Restaurant
	RestaurantPerformanceCache  map[string]*models.RestaurantPerformanceCache
	MenuItems                   map[string]*models.MenuItem
	WeatherState                *WeatherState
	CurrentTime                 time.Time
	EventQueue                  *models.EventQueue
	rngMutex                    sync.Mutex
	Rng                         *rand.Rand
}

func NewSimulator(config *models.Config, dbPool *pgxpool.Pool) *Simulator {
	sim := &Simulator{
		Config:                     config,
		CurrentTime:                config.StartDate,
		UserRepo:                   postgres.NewUserRepository(dbPool),
		RestaurantRepo:             postgres.NewRestaurantRepository(dbPool),
		MenuItemRepo:               postgres.NewMenuItemRepository(dbPool),
		DeliveryPartnerRepo:        postgres.NewDeliveryPartnerRepository(dbPool),
		Restaurants:                make(map[string]*models.Restaurant),
		RestaurantPerformanceCache: make(map[string]*models.RestaurantPerformanceCache),
		MenuItems:                  make(map[string]*models.MenuItem),
		Rng:                        rand.New(rand.NewSource(time.Now().UnixNano())),
		Users:                      make([]*models.User, config.InitialUsers),
		DeliveryPartners:           make([]*models.DeliveryPartner, config.InitialPartners),
		EventQueue:                 models.NewEventQueue(),
	}
	return sim
}

func (s *Simulator) initializeData(ctx context.Context) error {
	if s.Config.CleanSeed {
		if err := s.cleanAllData(ctx); err != nil {
			return err
		}
	}

	const batchSize = 1000

	// initialise users
	userCount, err := s.UserRepo.Count(ctx)
	if err != nil {
		return err
	}

	if userCount < s.Config.InitialUsers {
		userFactory := &factories.UserFactory{}
		usersToCreate := s.Config.InitialUsers - userCount

		users := make([]*models.User, 0, usersToCreate)

		for i := 0; i < usersToCreate; i++ {
			user := userFactory.CreateUser(s.Config)
			users = append(users, user)
		}

		for i := 0; i < len(users); i += batchSize {
			end := i + batchSize
			if end > len(users) {
				end = len(users)
			}

			batch := users[i:end]
			if err := s.UserRepo.BulkCreate(ctx, batch); err != nil {
				return fmt.Errorf("failed to bulk create users: %w", err)
			}
		}

		log.Printf("Created %d users in batches of %d", usersToCreate, batchSize)
	}

	s.Users, err = s.UserRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	// initialise restaurants
	restaurantCount, err := s.RestaurantRepo.Count(ctx)
	if err != nil {
		return err
	}

	var newRestaurants []*models.Restaurant
	if restaurantCount < s.Config.InitialRestaurants {
		restaurantFactory := &factories.RestaurantFactory{}
		restaurantsToCreate := s.Config.InitialRestaurants - restaurantCount
		newRestaurants = make([]*models.Restaurant, 0, restaurantsToCreate)

		for i := 0; i < restaurantsToCreate; i++ {
			restaurant := restaurantFactory.CreateRestaurant(s.Config)
			newRestaurants = append(newRestaurants, restaurant)
		}

		for i := 0; i < len(newRestaurants); i += batchSize {
			end := i + batchSize
			if end > len(newRestaurants) {
				end = len(newRestaurants)
			}
			if err := s.RestaurantRepo.BulkCreate(ctx, newRestaurants[i:end]); err != nil {
				return fmt.Errorf("failed to bulk create restaurants: %w", err)
			}
		}
		log.Printf("Created %d restaurants in batches of %d", restaurantsToCreate, batchSize)
	}

	s.Restaurants, err = s.RestaurantRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	// initialise MenuItems slice for each restaurant if not already initialised
	for _, restaurant := range s.Restaurants {
		if restaurant.MenuItems == nil {
			restaurant.MenuItems = make([]string, 0)
		}
	}

	// initialise menu items
	fake := faker.New()
	menuItemCount, err := s.MenuItemRepo.Count(ctx)
	if err != nil {
		return err
	}

	if menuItemCount == 0 {
		var menuItems []*models.MenuItem
		menuItemFactory := &factories.MenuItemFactory{}

		for _, restaurant := range s.Restaurants {
			itemCount := fake.IntBetween(10, 30)
			for i := 0; i < itemCount; i++ {
				menuItem := menuItemFactory.CreateMenuItem(restaurant, s.Config)
				menuItems = append(menuItems, &menuItem)
				s.Restaurants[restaurant.ID].MenuItems = append(s.Restaurants[restaurant.ID].MenuItems, menuItem.ID)
			}
		}

		for i := 0; i < len(menuItems); i += batchSize {
			end := i + batchSize
			if end > len(menuItems) {
				end = len(menuItems)
			}
			if err := s.MenuItemRepo.BulkCreate(ctx, menuItems[i:end]); err != nil {
				return fmt.Errorf("failed to bulk create menu items: %w", err)
			}
		}
	}

	s.MenuItems, err = s.MenuItemRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	// initialise delivery partners
	partnerCount, err := s.DeliveryPartnerRepo.Count(ctx)
	if err != nil {
		return err
	}

	if partnerCount < s.Config.InitialPartners {
		deliveryPartnerFactory := &factories.DeliveryPartnerFactory{}
		partners := make([]*models.DeliveryPartner, 0, s.Config.InitialPartners-partnerCount)

		for i := 0; i < s.Config.InitialPartners-partnerCount; i++ {
			partner := deliveryPartnerFactory.CreateDeliveryPartner(s.Config)
			partners = append(partners, partner)
		}

		for i := 0; i < len(partners); i += batchSize {
			end := i + batchSize
			if end > len(partners) {
				end = len(partners)
			}
			if err := s.DeliveryPartnerRepo.BulkCreate(ctx, partners[i:end]); err != nil {
				return fmt.Errorf("failed to bulk create delivery partners: %w", err)
			}
		}
	}

	s.DeliveryPartners, err = s.DeliveryPartnerRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	// initialise traffic conditions
	s.initializeTrafficConditions()

	// initialise maps
	s.OrdersByUser = make(map[string][]models.Order)
	s.CompletedOrdersByRestaurant = make(map[string][]models.Order)

	return nil
}

func (s *Simulator) cleanAllData(ctx context.Context) error {
	if err := s.MenuItemRepo.DeleteAll(ctx); err != nil {
		return err
	}
	if err := s.UserRepo.DeleteAll(ctx); err != nil {
		return err
	}
	if err := s.RestaurantRepo.DeleteAll(ctx); err != nil {
		return err
	}
	if err := s.DeliveryPartnerRepo.DeleteAll(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Simulator) processEvent(event *models.Event) {
	switch event.Type {
	case models.EventPlaceOrder:
		s.handlePlaceOrder(event.Data.(*models.User))
	case models.EventPrepareOrder:
		s.handlePrepareOrder(event.Data.(*models.Order))
	case models.EventOrderReady:
		s.handleOrderReady(event.Data.(*models.Order))
	case models.EventPickUpOrder:
		s.handlePickUpOrder(event)
	case models.EventAssignDeliveryPartner:
		s.handleAssignDeliveryPartner(event)
	case models.EventUpdatePartnerLocation:
		s.handleUpdatePartnerLocation(event.Data.(*models.PartnerLocationUpdate))
	case models.EventOrderInTransit:
		s.handleOrderInTransit(event.Data.(*models.Order))
	case models.EventCheckDeliveryStatus:
		s.handleCheckDeliveryStatus(event.Data.(*models.Order))
	case models.EventDeliverOrder:
		s.handleDeliverOrder(event.Data.(*models.Order))
	case models.EventCancelOrder:
		s.handleCancelOrder(event.Data.(*models.Order))
	case models.EventUpdateUserBehaviour:
		s.handleUpdateUserBehaviour(event.Data.(*models.UserBehaviourUpdate))
	case models.EventUpdateRestaurantStatus:
		s.handleUpdateRestaurantStatus(event.Data.(*models.Restaurant))
	case models.EventGenerateReview:
		s.handleGenerateReview(event.Data.(*models.Order))

	}
}

func (s *Simulator) simulateTimeStep() {
	// apply growth rate to user and partner base
	//growthRate := s.calculateGrowthRate(s.Config.UserGrowthRate, s.CurrentTime)
	//s.updatePopulationWithGrowth(growthRate)
	s.updateTrafficConditions()
	s.updateUserBehaviour()
	s.updateRestaurantStatus()
	s.updateDeliveryPartnerLocations()
	s.generateOrdersWithProbability()
	s.updateOrderStatuses()
}

func (s *Simulator) showProgress(eventsCount int) {
	if eventsCount%1000 == 0 {
		log.Printf("Current time: %s, Events processed: %d", s.CurrentTime.Format(time.RFC3339), eventsCount)
	}
}

func (s *Simulator) serializeEvent(event models.Event) (models.EventMessage, error) {
	var eventData interface{}
	var topic string

	baseEvent := NewBaseEvent(event.Type, event.Time)

	switch event.Type {
	case models.EventPlaceOrder:
		user := event.Data.(*models.User)
		// create an order for this user
		order, err := s.createAndAddOrder(user)
		if err != nil {
			return models.EventMessage{}, fmt.Errorf("failed to create order: %w", err)
		}
		eventData = OrderPlacedEvent{
			Timestamp:         event.Time.Unix(),
			OrderID:           order.ID,
			Items:             order.Items,
			TotalAmount:       order.TotalAmount,
			OrderPlacedAt:     order.OrderPlacedAt.Unix(),
			CustomerID:        order.CustomerID,
			RestaurantID:      order.RestaurantID,
			DeliveryPartnerID: order.DeliveryPartnerID,
			DeliveryCost:      order.DeliveryCost,
			PaymentMethod:     order.PaymentMethod,
			Address:           user.Location,
			ReviewGenerated:   order.ReviewGenerated,
			WeatherCondition:  s.getCurrentWeather(),
			Temperature:       s.getCurrentTemperature(),
		}
		topic = "order_placed_events"

	case models.EventPrepareOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID

		eventData = OrderPreparationEvent{
			BaseEvent:     baseEvent,
			OrderID:       order.ID,
			Status:        order.Status,
			PrepStartTime: order.PrepStartTime.Unix(),
		}
		topic = "order_preparation_events"

	case models.EventOrderReady:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID

		eventData = OrderReadyEvent{
			BaseEvent:  baseEvent,
			OrderID:    order.ID,
			Status:     order.Status,
			PickupTime: order.PickupTime.Unix(),
		}
		topic = "order_ready_events"

	case models.EventAssignDeliveryPartner:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.DeliveryID = order.DeliveryPartnerID

		eventData = DeliveryPartnerAssignmentEvent{
			BaseEvent:           baseEvent,
			OrderID:             order.ID,
			Status:              order.Status,
			EstimatedPickupTime: order.EstimatedPickupTime.Unix(),
		}
		topic = "delivery_partner_assignment_events"

	case models.EventPickUpOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.DeliveryID = order.DeliveryPartnerID

		eventData = OrderPickupEvent{
			BaseEvent:             baseEvent,
			OrderID:               order.ID,
			Status:                order.Status,
			PickupTime:            order.PickupTime.Unix(),
			EstimatedDeliveryTime: order.EstimatedDeliveryTime.Unix(),
		}
		topic = "order_pickup_events"

	case models.EventUpdatePartnerLocation:
		update := event.Data.(*models.PartnerLocationUpdate)
		partner := s.getDeliveryPartner(update.PartnerID)
		if partner == nil {
			return models.EventMessage{}, fmt.Errorf("partner not found: %s", update.PartnerID)
		}

		eventData = PartnerLocationUpdateEvent{
			Timestamp:        event.Time.Unix(),
			EventType:        event.Type,
			PartnerID:        update.PartnerID,
			OrderID:          update.OrderID,
			NewLocation:      models.Location{Lat: update.NewLocation.Lat, Lon: update.NewLocation.Lon},
			CurrentOrder:     partner.CurrentOrderID,
			Status:           partner.Status,
			UpdateTime:       s.CurrentTime.Unix(),
			Speed:            update.Speed,
			Experience:       partner.Experience,
			Rating:           partner.Rating,
			WeatherCondition: s.getCurrentWeather(),
			TrafficDensity:   s.getCurrentTrafficMultiplier(),
			AreaType:         s.getLocationCluster(update.NewLocation),
		}
		topic = "partner_location_events"

	case models.EventOrderInTransit:
		order := event.Data.(*models.Order)
		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		if partner == nil {
			return models.EventMessage{}, fmt.Errorf("delivery partner not found for order %s", order.ID)
		}

		eventData = OrderInTransitEvent{
			BaseEvent:             baseEvent,
			OrderID:               order.ID,
			DeliveryPartnerID:     order.DeliveryPartnerID,
			CustomerID:            order.CustomerID,
			CurrentLocation:       models.Location{Lat: partner.CurrentLocation.Lat, Lon: partner.CurrentLocation.Lon},
			EstimatedDeliveryTime: order.EstimatedDeliveryTime.Unix(),
			PickupTime:            order.PickupTime.Unix(),
			Status:                order.Status,
		}
		topic = "order_in_transit_events"

	case models.EventCheckDeliveryStatus:
		order, ok := event.Data.(*models.Order)
		if !ok {
			return models.EventMessage{}, fmt.Errorf("invalid data type for EventCheckDeliveryStatus")
		}
		baseEvent.UserID = order.CustomerID
		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.DeliveryID = order.DeliveryPartnerID

		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		var currentLocation models.Location
		if partner != nil {
			currentLocation = models.Location{Lat: partner.CurrentLocation.Lat, Lon: partner.CurrentLocation.Lon}
		}

		eventData = DeliveryStatusCheckEvent{
			BaseEvent:             baseEvent,
			OrderID:               order.ID,
			Status:                order.Status,
			EstimatedDeliveryTime: order.EstimatedDeliveryTime.Unix(),
			CurrentLocation:       currentLocation,
			NextCheckTime:         event.Time.Unix(),
		}
		topic = "delivery_status_check_events"

	case models.EventDeliverOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.DeliveryID = order.DeliveryPartnerID
		baseEvent.UserID = order.CustomerID

		eventData = OrderDeliveryEvent{
			BaseEvent:             baseEvent,
			OrderID:               order.ID,
			Status:                order.Status,
			EstimatedDeliveryTime: safeUnixTime(order.EstimatedDeliveryTime),
			ActualDeliveryTime:    safeUnixTime(order.ActualDeliveryTime),
		}
		topic = "order_delivery_events"

	case models.EventCancelOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.UserID = order.CustomerID

		eventData = OrderCancellationEvent{
			BaseEvent:        baseEvent,
			OrderID:          order.ID,
			Status:           order.Status,
			CancellationTime: s.CurrentTime.Unix(),
		}
		topic = "order_cancellation_events"

	case models.EventUpdateUserBehaviour:
		update := event.Data.(*models.UserBehaviourUpdate)
		user := s.getUser(update.UserID)
		if user == nil {
			return models.EventMessage{}, fmt.Errorf("user not found: %s", update.UserID)
		}

		timestamp := event.Time.Unix()
		eventType := event.Type
		userId := update.UserID
		orderFrequency := update.OrderFrequency
		lastOrderTime := safeUnixTime(user.LastOrderTime)
		locationType := s.getLocationCluster(user.Location)

		userBehaviorEvent := UserBehaviourUpdateEvent{
			Timestamp:          &timestamp,
			EventType:          &eventType,
			UserID:             &userId,
			OrderFrequency:     &orderFrequency,
			OrderPatterns:      &user.PurchasePatterns,
			CuisinePreferences: &user.Preferences,
			LocationType:       &locationType,
		}

		// only include LastOrderTime if it's not the zero value
		if !user.LastOrderTime.IsZero() {
			lastOrderTime = safeUnixTime(user.LastOrderTime)
			userBehaviorEvent.LastOrderTime = &lastOrderTime
		}

		eventData = userBehaviorEvent
		topic = "user_behaviour_events"

	case models.EventUpdateRestaurantStatus:
		restaurant := event.Data.(*models.Restaurant)
		baseEvent.RestaurantID = restaurant.ID

		prepTime := restaurant.PrepTime
		if math.IsNaN(prepTime) {
			prepTime = restaurant.MinPrepTime
		}

		capacity := restaurant.Capacity
		eventData = RestaurantStatusUpdateEvent{
			BaseEvent: baseEvent,
			Capacity:  &capacity,
			PrepTime:  &prepTime,
		}
		topic = "restaurant_status_events"

	case models.EventGenerateReview:
		order := event.Data.(*models.Order)

		baseEvent.RestaurantID = order.RestaurantID
		baseEvent.DeliveryID = order.DeliveryPartnerID
		baseEvent.UserID = order.CustomerID
		// create the review
		review := s.createReview(order)

		// add the review to the simulator's reviews
		s.Reviews = append(s.Reviews, review)

		// update ratings based on the review
		s.updateRatings(review)

		// update restaurant reputation
		restaurant := s.getRestaurant(order.RestaurantID)
		s.updateRestaurantReputation(restaurant, review)

		eventData = ReviewEvent{
			BaseEvent:         baseEvent,
			ReviewID:          review.ID,
			OrderID:           review.OrderID,
			CustomerID:        review.CustomerID,
			DeliveryPartnerID: review.DeliveryPartnerID,
			FoodRating:        review.FoodRating,
			DeliveryRating:    review.DeliveryRating,
			OverallRating:     review.OverallRating,
			Comment:           review.Comment,
			CreatedAt:         review.CreatedAt.Unix(),
			OrderTotal:        order.TotalAmount,
			DeliveryTime:      order.ActualDeliveryTime.Sub(order.OrderPlacedAt).Milliseconds(),
		}
		topic = "review_events"

	default:
		return models.EventMessage{}, fmt.Errorf("unknown event type: %v", event.Type)
	}

	// serialize the event to JSON
	data, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("Error serializing event: %v", err)
		return models.EventMessage{}, err
	}

	// return the event message
	return models.EventMessage{
		Topic:   topic,
		Message: data,
	}, nil
}

func (s *Simulator) cleanupSimulationState() {
	// create a map of valid order IDs
	validOrderIDs := make(map[string]bool)
	for _, order := range s.Orders {
		if order.Status != models.OrderStatusDelivered && order.Status != models.OrderStatusCancelled {
			validOrderIDs[order.ID] = true
		}
	}

	// check and correct partner statuses
	for i, partner := range s.DeliveryPartners {
		if partner.Status != models.PartnerStatusAvailable {
			order := s.getOrderByID(partner.CurrentOrderID)
			if order == nil || order.DeliveryPartnerID != partner.ID {
				log.Printf("Correcting inconsistent state: Partner %s has status %s but no valid current order. Resetting to available.",
					partner.ID, partner.Status)
				s.DeliveryPartners[i].Status = models.PartnerStatusAvailable
				s.DeliveryPartners[i].CurrentOrderID = ""
			}
		} else if partner.CurrentOrderID != "" {
			log.Printf("Correcting inconsistent state: Partner %s is available but has a current order. Clearing order ID.",
				partner.ID)
			s.DeliveryPartners[i].CurrentOrderID = ""
		}
	}

	// check and correct order assignments
	for i, order := range s.Orders {
		if order.Status != models.OrderStatusDelivered && order.Status != models.OrderStatusCancelled {
			if order.DeliveryPartnerID != "" {
				partner := s.getDeliveryPartner(order.DeliveryPartnerID)
				if partner == nil || partner.CurrentOrderID != order.ID {
					log.Printf("Correcting inconsistent state: Order %s assigned to non-existent or mismatched partner. Resetting.",
						order.ID)
					s.Orders[i].DeliveryPartnerID = ""
					// try to reassign the order
					s.assignDeliveryPartner(&s.Orders[i])
				}
			}
			if order.EstimatedDeliveryTime.IsZero() || order.EstimatedDeliveryTime.Before(s.CurrentTime) {
				log.Printf("Correcting invalid estimated delivery time for order %s", order.ID)
				s.Orders[i].EstimatedDeliveryTime = s.CurrentTime.Add(30 * time.Minute)
			}
		}
	}

	// check and correct invalid estimated delivery times
	for i, order := range s.Orders {
		if order.Status != models.OrderStatusDelivered && order.Status != models.OrderStatusCancelled {
			if order.EstimatedDeliveryTime.IsZero() || order.EstimatedDeliveryTime.Before(s.CurrentTime) {
				log.Printf("Correcting invalid estimated delivery time for order %s", order.ID)
				s.Orders[i].EstimatedDeliveryTime = s.CurrentTime.Add(30 * time.Minute)
			}
		}
	}
}

// event handlers
func (s *Simulator) handlePlaceOrder(user *models.User) {
	// Schedule next order for this user
	nextOrderTime := s.generateNextOrderTime(user)
	s.EventQueue.Enqueue(&models.Event{
		Time: nextOrderTime,
		Type: models.EventPlaceOrder,
		Data: user,
	})

	// Update user's order frequency
	user.OrderFrequency = s.adjustOrderFrequency(user)
}

func (s *Simulator) handlePrepareOrder(order *models.Order) {
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}

	// update order status
	order.Status = models.OrderStatusPreparing

	// estimate prep time
	prepTime := s.estimatePrepTime(restaurant, order.Items)

	// add some variability to prep time
	variability := 0.2 // 20% variability
	actualPrepTime := prepTime * (1 + (s.safeFloat64()*2-1)*variability)

	// calculate when the order will be ready
	readyTime := s.CurrentTime.Add(time.Duration(actualPrepTime) * time.Minute)

	// ensure prep time is reasonable
	maxPrepTime := 2 * time.Hour
	if readyTime.Sub(s.CurrentTime) > maxPrepTime {
		readyTime = s.CurrentTime.Add(maxPrepTime)
	}

	// ensure ready time is not before current time
	if readyTime.Before(s.CurrentTime) {
		readyTime = s.CurrentTime.Add(15 * time.Minute)
	}

	// update order
	order.PrepStartTime = s.CurrentTime
	order.PickupTime = readyTime

	// update restaurant orders
	restaurant.CurrentOrders = append(restaurant.CurrentOrders, *order)

	// schedule the next event (order ready)
	s.EventQueue.Enqueue(&models.Event{
		Time: readyTime,
		Type: models.EventOrderReady,
		Data: order,
	})

	// optionally, update restaurant metrics
	s.updateRestaurantMetrics(restaurant)

	log.Printf("Order %s preparation started at %s, estimated ready time: %s",
		order.ID, s.CurrentTime.Format(time.RFC3339), readyTime.Format(time.RFC3339))
}

func (s *Simulator) handleOrderReady(order *models.Order) {
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}

	// update order status
	order.Status = models.OrderStatusReady

	log.Printf("Order %s is ready for pickup at %s", order.ID, s.CurrentTime.Format(time.RFC3339))

	// if a delivery partner is already assigned, notify them
	if order.DeliveryPartnerID != "" {
		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		s.EventQueue.Enqueue(&models.Event{
			Time: s.CurrentTime, // try pickup immediately
			Type: models.EventPickUpOrder,
			Data: order,
		})
		if partner != nil {
			s.notifyDeliveryPartner(partner, order)
		} else {
			log.Printf("Warning: Assigned delivery partner %s not found for order %s", order.DeliveryPartnerID, order.ID)
		}
	} else {
		// if no delivery partner is assigned yet, try to assign one
		s.assignDeliveryPartner(order)
	}

	// update restaurant's current orders
	for i, currentOrder := range restaurant.CurrentOrders {
		if currentOrder.ID == order.ID {
			restaurant.CurrentOrders[i] = *order
			break
		}
	}

	//schedule the next event (pickup)
	// set a timeout for pickup. If not picked up within this time, reassign the order
	pickupTimeout := s.CurrentTime.Add(5 * time.Minute)
	s.EventQueue.Enqueue(&models.Event{
		Time: pickupTimeout,
		Type: models.EventPickUpOrder,
		Data: order,
	})

	// optionally, update restaurant metrics
	s.updateRestaurantMetrics(restaurant)
}

func (s *Simulator) handleAssignDeliveryPartner(event *models.Event) {
	order := event.Data.(*models.Order)

	// check if the order has already been assigned a delivery partner
	if order.DeliveryPartnerID != "" {
		log.Printf("Order %s already has a delivery partner assigned", order.ID)
		return
	}

	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}

	availablePartners := s.getAvailablePartnersNear(restaurant.Location)

	if len(availablePartners) == 0 {
		// if no partners are available, schedule a retry
		retryTime := s.CurrentTime.Add(2 * time.Minute)
		s.EventQueue.Enqueue(&models.Event{
			Time: retryTime,
			Type: models.EventAssignDeliveryPartner,
			Data: order,
		})
		return
	}

	selectedPartner := availablePartners[s.safeIntn(len(availablePartners))]

	// update both order and partner atomically
	if err := s.assignPartnerToOrder(selectedPartner, order); err != nil {
		log.Printf("Error assigning partner to order: %v", err)
		return
	}

	// calculate estimated pickup time
	estimatedPickupTime := s.estimateArrivalTime(selectedPartner.CurrentLocation, restaurant.Location)
	order.EstimatedPickupTime = estimatedPickupTime

	// schedule the pickup event
	s.EventQueue.Enqueue(&models.Event{
		Time: estimatedPickupTime,
		Type: models.EventPickUpOrder,
		Data: order,
	})

	log.Printf("Assigned delivery partner %s to order %s. Estimated pickup time: %s",
		selectedPartner.ID, order.ID, estimatedPickupTime.Format(time.RFC3339))

	s.notifyDeliveryPartner(selectedPartner, order)
}

func (s *Simulator) handlePickUpOrder(event *models.Event) {
	order := event.Data.(*models.Order)

	// verify the order status - allow both READY and PICKED_UP states
	if order.Status != models.OrderStatusReady && order.Status != models.OrderStatusPickedUp {
		log.Printf("Order %s cannot be picked up. Current status: %s", order.ID, order.Status)
		return
	}

	// get the assigned delivery partner
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	if partner == nil {
		log.Printf("Error: Delivery partner not found for order %s", order.ID)
		return
	}

	// check if the delivery partner is at the restaurant
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}

	if !s.isAtLocation(partner.CurrentLocation, restaurant.Location) {
		// if the partner is not at the restaurant, reschedule the pickup
		nextAttempt := s.CurrentTime.Add(2 * time.Minute)
		s.EventQueue.Enqueue(&models.Event{
			Time: nextAttempt,
			Type: models.EventPickUpOrder,
			Data: order,
		})
		return
	}

	// only update status if not already picked up
	if order.Status != models.OrderStatusPickedUp {
		order.Status = models.OrderStatusPickedUp
		order.PickupTime = s.CurrentTime
		partner.Status = models.PartnerStatusEnRouteDelivery

		// trigger the "order in transit" event
		s.EventQueue.Enqueue(&models.Event{
			Time: s.CurrentTime,
			Type: models.EventOrderInTransit,
			Data: order,
		})

		// estimate delivery time
		estimatedDeliveryTime := s.estimateDeliveryTime(partner, order)
		order.EstimatedDeliveryTime = estimatedDeliveryTime

		// schedule the first check event
		nextCheckTime := s.CurrentTime.Add(5 * time.Minute)
		s.EventQueue.Enqueue(&models.Event{
			Time: nextCheckTime,
			Type: models.EventCheckDeliveryStatus,
			Data: order,
		})

		log.Printf("Order %s picked up by partner %s. Estimated delivery time: %s",
			order.ID, partner.ID, estimatedDeliveryTime.Format(time.RFC3339))
	}
}

func (s *Simulator) handleCancelOrder(order *models.Order) {
	// update order status
	order.Status = models.OrderStatusCancelled

	// if a delivery partner was assigned, update their status
	if order.DeliveryPartnerID != "" {
		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		if partner != nil {
			partner.Status = models.PartnerStatusAvailable
			partner.CurrentOrderID = ""
		}
	}

	// if the order was being prepared, update restaurant status
	if order.Status == models.OrderStatusPreparing {
		restaurant := s.getRestaurant(order.RestaurantID)
		if restaurant != nil {
			// Remove the order from the restaurant's current orders
			for i, currentOrder := range restaurant.CurrentOrders {
				if currentOrder.ID == order.ID {
					restaurant.CurrentOrders = append(restaurant.CurrentOrders[:i], restaurant.CurrentOrders[i+1:]...)
					break
				}
			}
		}
	}

	log.Printf("Order %s cancelled at %s", order.ID, s.CurrentTime.Format(time.RFC3339))
}

func (s *Simulator) handleCheckDeliveryStatus(order *models.Order) {
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	user := s.getUser(order.CustomerID)

	if partner == nil || user == nil {
		log.Printf("Error: Cannot check delivery status for order %s. Missing partner or user info.", order.ID)
		return
	}

	distance := s.calculateDistance(partner.CurrentLocation, user.Location)
	log.Printf("Order %s: Distance to customer: %.2f km", order.ID, distance)

	if distance <= deliveryThreshold {
		// order has been delivered
		s.handleDeliverOrder(order)
		return
	} else {
		// calculate duration since last update
		duration := s.CurrentTime.Sub(partner.LastUpdateTime)

		// move the partner towards the customer
		partner.CurrentLocation = s.moveTowards(partner.CurrentLocation, user.Location, duration)
		partner.LastUpdateTime = s.CurrentTime

		// order is still in transit, schedule next check
		nextCheckTime := s.CurrentTime.Add(5 * time.Minute)
		s.EventQueue.Enqueue(&models.Event{
			Time: nextCheckTime,
			Type: models.EventCheckDeliveryStatus,
			Data: order,
		})
		log.Printf("Order %s still in transit. Next status check at: %s", order.ID, nextCheckTime.Format(time.RFC3339))
	}
}

func (s *Simulator) handleUpdateRestaurantStatus(restaurant *models.Restaurant) {
	// update restaurant metrics
	s.updateRestaurantMetrics(restaurant)

	// adjust restaurant capacity based on current conditions
	restaurant.Capacity = s.adjustRestaurantCapacity(restaurant)

	// update prep time based on current load
	restaurant.PrepTime = s.adjustPrepTime(restaurant)

	// schedule next update
	s.EventQueue.Enqueue(&models.Event{
		Time: s.CurrentTime.Add(15 * time.Minute), // update every 15 minutes
		Type: models.EventUpdateRestaurantStatus,
		Data: restaurant,
	})

	//log.Printf("Updated status for restaurant %s at %s. Capacity: %d -> %d, Prep time: %.2f -> %.2f",
	//	restaurant.ID, s.CurrentTime.Format(time.RFC3339),
	//	oldCapacity, restaurant.Capacity,
	//	oldPrepTime, restaurant.PrepTime)
}

func (s *Simulator) handleUpdatePartnerLocation(update *models.PartnerLocationUpdate) {
	partner := s.getDeliveryPartner(update.PartnerID)
	if partner != nil {
		partner.CurrentLocation = update.NewLocation
	}
}

func (s *Simulator) handleOrderInTransit(order *models.Order) {
	if order.Status != models.OrderStatusPickedUp {
		return
	}

	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	if partner == nil {
		log.Printf("Error: Delivery partner not found for order %s", order.ID)
		return
	}

	// ensure we're not scheduling the same event again
	if order.Status != models.OrderStatusInTransit {
		order.Status = models.OrderStatusInTransit
		order.InTransitTime = s.CurrentTime

		// schedule a check event
		nextCheckTime := s.CurrentTime.Add(5 * time.Minute) // check every 5 minutes
		s.EventQueue.Enqueue(&models.Event{
			Time: nextCheckTime,
			Type: models.EventCheckDeliveryStatus,
			Data: order,
		})

		log.Printf("Order %s is now in transit. Next status check at: %s",
			order.ID, nextCheckTime.Format(time.RFC3339))
	} else {
		log.Printf("Order %s is already in transit. Skipping.", order.ID)
	}
}

func (s *Simulator) handleDeliverOrder(order *models.Order) {
	// get the delivery partner
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	if partner == nil {
		log.Printf("Error: Delivery partner not found for order %s", order.ID)
		return
	}

	// get the user
	user := s.getUser(order.CustomerID)
	if user == nil {
		log.Printf("Error: User not found for order %s", order.ID)
		return
	}

	// verify partner has reached customer
	if !s.isAtLocation(partner.CurrentLocation, user.Location) {
		return
	}

	// update order status
	order.Status = models.OrderStatusDelivered
	order.ActualDeliveryTime = s.CurrentTime

	// update delivery partner status
	partner.Status = models.PartnerStatusAvailable
	partner.CurrentOrderID = ""

	// schedule review
	reviewTime := s.CurrentTime.Add(30 * time.Minute)
	s.EventQueue.Enqueue(&models.Event{
		Time: reviewTime,
		Type: models.EventGenerateReview,
		Data: order,
	})

	log.Printf("Order %s delivered to user %s at %s",
		order.ID, user.ID, s.CurrentTime.Format(time.RFC3339))

	// ensure this event is being serialized and written
	eventMsg, err := s.serializeEvent(models.Event{
		Time: s.CurrentTime,
		Type: models.EventDeliverOrder,
		Data: order,
	})

	if err != nil {
		log.Printf("Error serializing delivery event: %v", err)
	} else {
		output := s.determineOutputDestination()
		if err := output.WriteMessage(eventMsg.Topic, eventMsg.Message); err != nil {
			log.Printf("Failed to write delivery message: %v", err)
		}
	}
}

func (s *Simulator) handleUpdateUserBehaviour(update *models.UserBehaviourUpdate) {
	user := s.getUser(update.UserID)
	if user != nil {
		user.OrderFrequency = update.OrderFrequency
	}
}

func (s *Simulator) handleGenerateReview(order *models.Order) {
	// check if we should generate a review for this order
	if !s.shouldGenerateReview(order) {
		return
	}

	// enqueue the review generation event
	s.EventQueue.Enqueue(&models.Event{
		Time: s.CurrentTime,
		Type: models.EventGenerateReview,
		Data: order,
	})

	// set the ReviewGenerated flag to true
	order.ReviewGenerated = true

	log.Printf("Review generation for order %s scheduled. %.1f", order.ID)
}

func (s *Simulator) Run() {
	// initialize maps if nil
	if s.Restaurants == nil {
		s.Restaurants = make(map[string]*models.Restaurant)
	}
	if s.MenuItems == nil {
		s.MenuItems = make(map[string]*models.MenuItem)
	}
	if s.OrdersByUser == nil {
		s.OrdersByUser = make(map[string][]models.Order)
	}
	output := s.determineOutputDestination()
	ctx := context.Background()
	defer func() {
		if closer, ok := output.(io.Closer); ok {
			err := closer.Close()
			if err != nil {
				log.Printf("Error closing output: %v", err)
				return
			}
		}
	}()

	if err := s.initializeData(ctx); err != nil {
		log.Printf("Error initialising data: %v", err)
		return
	}

	log.Printf("Simulation starts from %s to %s\n", s.CurrentTime.Format(time.RFC3339), s.Config.EndDate.Format(time.RFC3339))

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	var eventsCount int
	var eventsCountMutex sync.Mutex

	// create a worker pool
	numWorkers := runtime.NumCPU() // use number of CPUs as worker count
	jobs := make(chan *models.Event, numWorkers)
	var wg sync.WaitGroup

	// start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for event := range jobs {
				s.processEvent(event)
				eventMsg, err := s.serializeEvent(*event)
				if err != nil {
					log.Printf("Error serializing event: %v", err)
					continue
				}
				if err := output.WriteMessage(eventMsg.Topic, eventMsg.Message); err != nil {
					log.Printf("Failed to write message: %v", err)
				}
				eventsCountMutex.Lock()
				eventsCount++
				eventsCountMutex.Unlock()
			}
		}()
	}

	totalDuration := s.Config.EndDate.Sub(s.CurrentTime)
	bar := progressbar.Default(100)

	for s.CurrentTime.Before(s.Config.EndDate) {
		select {
		case <-ticker.C:
			// process any events that are due
			for {
				nextEvent := s.EventQueue.Peek()
				if nextEvent == nil || nextEvent.Time.After(s.CurrentTime) {
					break
				}
				batch := s.EventQueue.DequeueBatch(100)
				for _, event := range batch {
					jobs <- event // send event to worker pool
				}
			}
			// run time-step simulation
			s.simulateTimeStep()

			// cancel stale orders and cleanup simulation state
			s.cancelStaleOrders()
			s.cleanupSimulationState()
			s.removeCompletedOrders()

			// show progress
			eventsCountMutex.Lock()
			s.showProgress(eventsCount)
			eventsCountMutex.Unlock()

			progress := float64(s.CurrentTime.Sub(s.Config.StartDate)) / float64(totalDuration)
			bar.Set(int(progress * 100))

			// advance simulation time
			s.CurrentTime = s.CurrentTime.Add(10 * time.Minute)

		default:
			// if there are no events to process and no time has passed,
			// we can sleep for a short duration to avoid busy-waiting
			time.Sleep(1 * time.Millisecond)
		}
	}
	// close the jobs channel and wait for all workers to finish
	close(jobs)
	wg.Wait()

	log.Printf("Simulation completed at %s\n", time.Now().UTC().Format(time.RFC3339))
	defer s.CloseKafkaProducer(output)
}
