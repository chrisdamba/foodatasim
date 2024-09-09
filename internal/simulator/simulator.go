package simulator

import (
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/chrisdamba/foodatasim/internal/factories"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
)

type OutputDestination interface {
	WriteMessage(topic string, msg []byte) error
}

type ConsoleOutput struct{}

type KafkaOutput struct {
	producer sarama.SyncProducer
}

type FileOutput struct {
	files    map[string]*os.File
	basePath string // Base directory for output files
}

// NewFileOutput creates a new FileOutput instance with initialized values.
func NewFileOutput(basePath string) *FileOutput {
	return &FileOutput{
		files:    make(map[string]*os.File),
		basePath: basePath,
	}
}

type Simulator struct {
	Config                      *models.Config
	Users                       []*models.User
	DeliveryPartners            []*models.DeliveryPartner
	TrafficConditions           []models.TrafficCondition
	Orders                      []models.Order
	Reviews                     []models.Review
	OrdersByUser                map[string][]models.Order
	CompletedOrdersByRestaurant map[string][]models.Order
	Restaurants                 map[string]*models.Restaurant
	MenuItems                   map[string]*models.MenuItem
	CurrentTime                 time.Time
	Rng                         *rand.Rand
	EventQueue                  *models.EventQueue
}

func NewSimulator(config *models.Config) *Simulator {
	sim := &Simulator{
		Config:           config,
		CurrentTime:      config.StartDate,
		Restaurants:      make(map[string]*models.Restaurant),
		MenuItems:        make(map[string]*models.MenuItem),
		Rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
		Users:            make([]*models.User, config.InitialUsers),
		DeliveryPartners: make([]*models.DeliveryPartner, config.InitialPartners),
		EventQueue:       models.NewEventQueue(),
	}
	return sim
}

func (f *FileOutput) WriteMessage(topic string, msg []byte) error {
	// Check if the file already exists in the map
	if _, ok := f.files[topic]; !ok {
		// If not, create the file
		filename := fmt.Sprintf("%s/%s.txt", f.basePath, topic)
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file for topic %s: %w", topic, err)
		}
		f.files[topic] = file
	}

	// Write the message to the corresponding file
	_, err := f.files[topic].Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message to topic %s: %w", topic, err)
	}

	return nil
}

func (k *KafkaOutput) WriteMessage(topic string, msg []byte) error {
	if k.producer == nil {
		return fmt.Errorf("Kafka producer is closed")
	}
	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(msg),
	})
	return err
}

func (c *ConsoleOutput) WriteMessage(topic string, msg []byte) error {
	// Create a formatted string that includes the topic
	output := fmt.Sprintf("[%s] %s\n", topic, string(msg))

	// Write the formatted string to stdout
	_, err := os.Stdout.Write([]byte(output))
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %w", err)
	}

	// Try to sync, but don't return an error if it fails
	_ = os.Stdout.Sync()

	return nil
}

func (s *Simulator) initializeData() {
	userFactory := &factories.UserFactory{}
	restaurantFactory := &factories.RestaurantFactory{}
	menuItemFactory := &factories.MenuItemFactory{}
	deliveryPartnerFactory := &factories.DeliveryPartnerFactory{}

	// initialise users
	for i := 0; i < s.Config.InitialUsers; i++ {
		s.Users[i] = userFactory.CreateUser(s.Config)
	}

	// initialise restaurants
	for i := 0; i < s.Config.InitialRestaurants; i++ {
		restaurant := restaurantFactory.CreateRestaurant(s.Config)
		s.Restaurants[restaurant.ID] = restaurant
	}

	// initialise delivery partners
	for i := 0; i < s.Config.InitialPartners; i++ {
		s.DeliveryPartners[i] = deliveryPartnerFactory.CreateDeliveryPartner(s.Config)
	}

	// initialise menu items
	fake := faker.New()
	for restaurantID, restaurant := range s.Restaurants {
		itemCount := fake.IntBetween(10, 30)
		for i := 0; i < itemCount; i++ {
			menuItem := menuItemFactory.CreateMenuItem(restaurant, s.Config)
			s.MenuItems[menuItem.ID] = &menuItem
			s.Restaurants[restaurantID].MenuItems = append(s.Restaurants[restaurantID].MenuItems, menuItem.ID)
		}
	}

	// initialise traffic conditions
	s.initializeTrafficConditions()

	// initialise maps
	s.OrdersByUser = make(map[string][]models.Order)
	s.CompletedOrdersByRestaurant = make(map[string][]models.Order)

	outputFolder := s.Config.OutputFolder
	if outputFolder == "" {
		outputFolder = "output" // Default to "output" if not specified
	}
	log.Printf("Cleaning and preparing output folder: %s", outputFolder)
	if err := s.serializeInitialDataToCSV(outputFolder); err != nil {
		log.Printf("Failed to serialize initial data to CSV: %v", err)
	} else {
		log.Printf("Initial data serialized to CSV in folder: %s", outputFolder)
	}
}

func (s *Simulator) processEvent(event *models.Event) {
	switch event.Type {
	case models.EventPlaceOrder:
		s.handlePlaceOrder(event.Data.(*models.User))
	case models.EventPrepareOrder:
		s.handlePrepareOrder(event.Data.(*models.Order))
	case models.EventOrderReady:
		s.handleOrderReady(event.Data.(*models.Order))
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

func (s *Simulator) Run() {
	output := s.determineOutputDestination()
	defer func() {
		if closer, ok := output.(io.Closer); ok {
			err := closer.Close()
			if err != nil {
				return
			}
		}
	}()

	s.initializeData()
	log.Printf("Simulation starts from %s to %s\n", s.CurrentTime.Format(time.RFC3339), s.Config.EndDate.Format(time.RFC3339))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var eventsCount int

	for s.CurrentTime.Before(s.Config.EndDate) {
		select {
		case <-ticker.C:
			// process any events that are due
			for {
				nextEvent := s.EventQueue.Peek()
				if nextEvent == nil || nextEvent.Time.After(s.CurrentTime) {
					break
				}
				event := s.EventQueue.Dequeue()
				if event != nil {
					s.processEvent(event)
					eventsCount++

					// serialize and write the event
					eventMsg, err := s.serializeEvent(*event)
					if err != nil {
						log.Printf("Error serializing event: %v", err)
						continue
					}
					if err := output.WriteMessage(eventMsg.Topic, eventMsg.Message); err != nil {
						log.Printf("Failed to write message: %v", err)
					}
				}
			}
			// run time-step simulation
			s.simulateTimeStep()

			// cancel stale orders and cleanup simulation state
			s.cancelStaleOrders()
			s.cleanupSimulationState()
			s.removeCompletedOrders()

			// show progress
			s.showProgress(eventsCount)

			// advance simulation time
			s.CurrentTime = s.CurrentTime.Add(1 * time.Minute)

		default:
			// if there are no events to process and no time has passed,
			// we can sleep for a short duration to avoid busy-waiting
			time.Sleep(10 * time.Millisecond)
		}
	}

	log.Printf("Simulation completed at %s\n", time.Now().UTC().Format(time.RFC3339))
}

func (s *Simulator) simulateTimeStep() {
	s.updateTrafficConditions()
	s.generateOrders()
	s.updateOrderStatuses()
	s.updateDeliveryPartnerLocations()
	s.updateUserBehaviour()
	s.updateRestaurantStatus()
}

func createKafkaProducer(brokerList []string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Successes = true
	config.Net.DialTimeout = 30 * time.Second
	config.Net.ReadTimeout = 30 * time.Second
	config.Net.WriteTimeout = 30 * time.Second

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	return producer, nil
}

func (s *Simulator) determineOutputDestination() OutputDestination {
	if s.Config.KafkaEnabled {
		brokerList := strings.Split(s.Config.KafkaBrokerList, ",")
		producer, err := createKafkaProducer(brokerList)
		if err != nil {
			log.Fatalf("Failed to create Kafka producer: %s", err)
		}
		return &KafkaOutput{producer: producer}
	} else if s.Config.OutputFile != "" {
		return NewFileOutput(s.Config.OutputFile)
	}
	return &ConsoleOutput{}
}

func (s *Simulator) showProgress(eventsCount int) {
	if eventsCount%1000 == 0 {
		log.Printf("Current time: %s, Events processed: %d", s.CurrentTime.Format(time.RFC3339), eventsCount)
	}
}

func (s *Simulator) serializeEvent(event models.Event) (models.EventMessage, error) {
	var topic string
	var eventData interface{}

	// base event structure that all events will have
	type BaseEvent struct {
		Timestamp    int64  `json:"timestamp"`
		EventType    string `json:"eventType"`
		UserID       string `json:"userId,omitempty"`
		RestaurantID string `json:"restaurantId,omitempty"`
		DeliveryID   string `json:"deliveryPartnerId,omitempty"`
	}

	baseEvent := BaseEvent{
		Timestamp: event.Time.Unix(),
		EventType: event.Type,
	}

	switch event.Type {
	case models.EventPlaceOrder:
		user := event.Data.(*models.User)
		baseEvent.UserID = user.ID
		// create an order for this user
		order, err := s.createAndAddOrder(user)
		if err != nil {
			return models.EventMessage{}, fmt.Errorf("failed to create order: %w", err)
		}
		baseEvent.RestaurantID = order.RestaurantID

		eventData = struct {
			BaseEvent
			OrderID       string   `json:"orderId"`
			Items         []string `json:"itemIds"`
			TotalAmount   float64  `json:"totalAmount"`
			Status        string   `json:"status"`
			OrderPlacedAt int64    `json:"orderPlacedAt"`
		}{
			BaseEvent:     baseEvent,
			OrderID:       order.ID,
			Items:         order.Items,
			TotalAmount:   order.TotalAmount,
			Status:        order.Status,
			OrderPlacedAt: order.OrderPlacedAt.Unix(),
		}
		topic = "order_placed_events"

	case models.EventPrepareOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID

		eventData = struct {
			BaseEvent
			OrderID       string `json:"orderId"`
			Status        string `json:"status"`
			PrepStartTime int64  `json:"prepStartTime"`
		}{
			BaseEvent:     baseEvent,
			OrderID:       order.ID,
			Status:        order.Status,
			PrepStartTime: order.PrepStartTime.Unix(),
		}
		topic = "order_preparation_events"

	case models.EventOrderReady:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID

		eventData = struct {
			BaseEvent
			OrderID    string `json:"orderId"`
			Status     string `json:"status"`
			PickupTime int64  `json:"pickupTime"`
		}{
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

		eventData = struct {
			BaseEvent
			OrderID             string `json:"orderId"`
			Status              string `json:"status"`
			EstimatedPickupTime int64  `json:"estimatedPickupTime"`
		}{
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

		eventData = struct {
			BaseEvent
			OrderID               string `json:"orderId"`
			Status                string `json:"status"`
			PickupTime            int64  `json:"pickupTime"`
			EstimatedDeliveryTime int64  `json:"estimatedDeliveryTime"`
		}{
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

		eventData = struct {
			BaseEvent
			PartnerID    string          `json:"partnerId"`
			OrderID      string          `json:"orderId,omitempty"`
			NewLocation  models.Location `json:"newLocation"`
			CurrentOrder string          `json:"currentOrder,omitempty"`
			Status       string          `json:"status"`
			UpdateTime   int64           `json:"updateTime"`
			Speed        float64         `json:"speed,omitempty"`
		}{
			BaseEvent:    baseEvent,
			PartnerID:    update.PartnerID,
			OrderID:      update.OrderID,
			NewLocation:  update.NewLocation,
			CurrentOrder: partner.CurrentOrderID,
			Status:       partner.Status,
			UpdateTime:   s.CurrentTime.Unix(),
			Speed:        update.Speed,
		}
		topic = "partner_location_events"

	case models.EventOrderInTransit:
		order := event.Data.(*models.Order)
		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		if partner == nil {
			return models.EventMessage{}, fmt.Errorf("delivery partner not found for order %s", order.ID)
		}

		eventData = struct {
			BaseEvent
			OrderID               string          `json:"orderId"`
			DeliveryPartnerID     string          `json:"deliveryPartnerId"`
			RestaurantID          string          `json:"restaurantId"`
			CustomerID            string          `json:"customerId"`
			CurrentLocation       models.Location `json:"currentLocation"`
			EstimatedDeliveryTime int64           `json:"estimatedDeliveryTime"`
			PickupTime            int64           `json:"pickupTime"`
			Status                string          `json:"status"`
		}{
			BaseEvent:             baseEvent,
			OrderID:               order.ID,
			DeliveryPartnerID:     order.DeliveryPartnerID,
			RestaurantID:          order.RestaurantID,
			CustomerID:            order.CustomerID,
			CurrentLocation:       partner.CurrentLocation,
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
			currentLocation = partner.CurrentLocation
		}

		eventData = struct {
			BaseEvent
			OrderID               string          `json:"orderId"`
			Status                string          `json:"status"`
			EstimatedDeliveryTime int64           `json:"estimatedDeliveryTime"`
			CurrentLocation       models.Location `json:"currentLocation"`
			NextCheckTime         int64           `json:"nextCheckTime"`
		}{
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

		eventData = struct {
			BaseEvent
			OrderID               string `json:"orderId"`
			Status                string `json:"status"`
			EstimatedDeliveryTime int64  `json:"estimatedDeliveryTime"`
			ActualDeliveryTime    int64  `json:"actualDeliveryTime"`
		}{
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

		eventData = struct {
			BaseEvent
			OrderID          string `json:"orderId"`
			Status           string `json:"status"`
			CancellationTime int64  `json:"cancellationTime"`
		}{
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

		userBehaviorEvent := struct {
			BaseEvent
			UserID         string  `json:"userId"`
			OrderFrequency float64 `json:"orderFrequency"`
			LastOrderTime  int64   `json:"lastOrderTime,omitempty"`
		}{
			BaseEvent:      baseEvent,
			UserID:         update.UserID,
			OrderFrequency: update.OrderFrequency,
		}

		// only include LastOrderTime if it's not the zero value
		if !user.LastOrderTime.IsZero() {
			userBehaviorEvent.LastOrderTime = user.LastOrderTime.Unix()
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

		eventData = struct {
			BaseEvent
			Capacity int     `json:"capacity"`
			PrepTime float64 `json:"prepTime"`
		}{
			BaseEvent: baseEvent,
			Capacity:  restaurant.Capacity,
			PrepTime:  prepTime,
		}
		topic = "restaurant_status_events"

	case models.EventGenerateReview:
		order := event.Data.(*models.Order)

		// create the review
		review := s.createReview(order)

		// add the review to the simulator's reviews
		s.Reviews = append(s.Reviews, review)

		// update ratings based on the review
		s.updateRatings(review)

		eventData = struct {
			BaseEvent
			ReviewID          string  `json:"reviewId"`
			OrderID           string  `json:"orderId"`
			CustomerID        string  `json:"customerId"`
			RestaurantID      string  `json:"restaurantId"`
			DeliveryPartnerID string  `json:"deliveryPartnerId"`
			FoodRating        float64 `json:"foodRating"`
			DeliveryRating    float64 `json:"deliveryRating"`
			OverallRating     float64 `json:"overallRating"`
			Comment           string  `json:"comment"`
			CreatedAt         int64   `json:"createdAt"`
			OrderTotal        float64 `json:"orderTotal"`
			DeliveryTime      int64   `json:"deliveryTime"`
		}{
			BaseEvent:         baseEvent,
			ReviewID:          review.ID,
			OrderID:           review.OrderID,
			CustomerID:        review.CustomerID,
			RestaurantID:      review.RestaurantID,
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
	actualPrepTime := prepTime * (1 + (s.Rng.Float64()*2-1)*variability)

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

	// Optionally, update restaurant metrics
	s.updateRestaurantMetrics(restaurant)

	// Log the event
	log.Printf("Order %s preparation started at %s, estimated ready time: %s",
		order.ID, s.CurrentTime.Format(time.RFC3339), readyTime.Format(time.RFC3339))
}

func (s *Simulator) handleOrderReady(order *models.Order) {
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}

	// Update order status
	order.Status = models.OrderStatusReady

	// Log the event
	log.Printf("Order %s is ready for pickup at %s", order.ID, s.CurrentTime.Format(time.RFC3339))

	// If a delivery partner is already assigned, notify them
	if order.DeliveryPartnerID != "" {
		partner := s.getDeliveryPartner(order.DeliveryPartnerID)
		if partner != nil {
			s.notifyDeliveryPartner(partner, order)
		} else {
			log.Printf("Warning: Assigned delivery partner %s not found for order %s", order.DeliveryPartnerID, order.ID)
		}
	} else {
		// If no delivery partner is assigned yet, try to assign one
		s.assignDeliveryPartner(order)
	}

	// Update restaurant's current orders
	for i, currentOrder := range restaurant.CurrentOrders {
		if currentOrder.ID == order.ID {
			restaurant.CurrentOrders[i] = *order
			break
		}
	}

	// Schedule the next event (pickup)
	// We'll set a timeout for pickup. If not picked up within this time, we'll reassign the order
	pickupTimeout := s.CurrentTime.Add(15 * time.Minute)
	s.EventQueue.Enqueue(&models.Event{
		Time: pickupTimeout,
		Type: models.EventPickUpOrder,
		Data: order,
	})

	// Optionally, update restaurant metrics
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
		log.Printf("No available delivery partners for order %s, scheduling retry at %s",
			order.ID, retryTime.Format(time.RFC3339))
		return
	}

	// select the best partner (for now, just select randomly)
	selectedPartner := availablePartners[s.Rng.Intn(len(availablePartners))]

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

	// notify the delivery partner (in a real system, this would send a notification)
	s.notifyDeliveryPartner(selectedPartner, order)
}

func (s *Simulator) handlePickUpOrder(event *models.Event) {
	order := event.Data.(*models.Order)

	// verify the order status
	if order.Status != models.OrderStatusReady {
		log.Printf("Error: Order %s is not ready for pickup. Current status: %s", order.ID, order.Status)
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
		log.Printf("Delivery partner not at restaurant. Rescheduling pickup for order %s at %s",
			order.ID, nextAttempt.Format(time.RFC3339))
		return
	}

	// update order status
	order.Status = models.OrderStatusPickedUp
	order.PickupTime = s.CurrentTime

	// update delivery partner status
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
	nextCheckTime := s.CurrentTime.Add(5 * time.Minute) // check every 5 minutes
	s.EventQueue.Enqueue(&models.Event{
		Time: nextCheckTime,
		Type: models.EventCheckDeliveryStatus,
		Data: order,
	})

	log.Printf("Order %s picked up by partner %s. Estimated delivery time: %s, Next status check at: %s",
		order.ID, partner.ID, estimatedDeliveryTime.Format(time.RFC3339), nextCheckTime.Format(time.RFC3339))
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

	// update order status
	order.Status = models.OrderStatusDelivered
	order.ActualDeliveryTime = s.CurrentTime

	// update delivery partner status
	partner.Status = models.PartnerStatusAvailable
	partner.CurrentOrderID = ""

	// generate a review event
	s.EventQueue.Enqueue(&models.Event{
		Time: s.CurrentTime.Add(30 * time.Minute), // Assume user leaves review after 30 minutes
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
	// Check if we should generate a review for this order
	if !s.shouldGenerateReview(order) {
		return
	}

	// Enqueue the review generation event
	s.EventQueue.Enqueue(&models.Event{
		Time: s.CurrentTime,
		Type: models.EventGenerateReview,
		Data: order,
	})

	// Set the ReviewGenerated flag to true
	order.ReviewGenerated = true

	log.Printf("Review generation for order %s scheduled. %.1f", order.ID)
}
