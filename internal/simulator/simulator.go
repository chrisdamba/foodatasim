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
	Reviews                     []models.Review
	TrafficConditions           []models.TrafficCondition
	Orders                      []models.Order
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

	// Ensure the output is immediately visible
	err = os.Stdout.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync stdout: %w", err)
	}

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
	for _, restaurant := range s.Restaurants {
		itemCount := fake.IntBetween(10, 30)
		for i := 0; i < itemCount; i++ {
			menuItem := menuItemFactory.CreateMenuItem(restaurant)
			s.MenuItems[menuItem.ID] = &menuItem
			restaurant.MenuItems = append(restaurant.MenuItems, menuItem.ID)
		}
	}

	// initialise traffic conditions
	s.initializeTrafficConditions()

	// initialise maps
	s.OrdersByUser = make(map[string][]models.Order)
	s.CompletedOrdersByRestaurant = make(map[string][]models.Order)
}

func (s *Simulator) processEvent(event *models.Event) {
	switch event.Type {
	case models.EventPlaceOrder:
		s.handlePlaceOrder(event.Data.(*models.User))
	case models.EventPrepareOrder:
		s.handlePrepareOrder(event.Data.(*models.Order))
	case models.EventOrderReady:
		s.handleOrderReady(event.Data.(*models.Order))

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
			// Process any events that are due
			for {
				nextEvent := s.EventQueue.Peek()
				if nextEvent == nil || nextEvent.Time.After(s.CurrentTime) {
					break
				}
				event := s.EventQueue.Dequeue()
				if event != nil {
					s.processEvent(event)
					eventsCount++

					// Serialize and write the event
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
			//for nil != s.EventQueue.Peek() && s.EventQueue.Peek().Time.Before(s.CurrentTime) {
			//	event := s.EventQueue.Dequeue()
			//	s.processEvent(event)
			//	eventsCount++
			//
			//	// Generate output for the event
			//	eventMsg, err := s.serializeEvent(event)
			//	if err != nil {
			//		log.Printf("Error serializing event: %v", err)
			//		continue
			//	}
			//	if err := output.WriteMessage(eventMsg.Topic, eventMsg.Message); err != nil {
			//		log.Printf("Failed to write message: %v", err)
			//	}
			//}

			// Run time-step simulation
			s.simulateTimeStep()

			// Show progress
			s.showProgress(eventsCount)

			// Advance simulation time
			s.CurrentTime = s.CurrentTime.Add(1 * time.Minute)

		default:
			// If there are no events to process and no time has passed,
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
	s.updateUserBehavior()
	s.updateRestaurantStatus()
}

func (s *Simulator) determineOutputDestination() OutputDestination {
	if s.Config.KafkaEnabled {
		brokerList := strings.Split(s.Config.KafkaBrokerList, ",")
		producer, err := sarama.NewSyncProducer(brokerList, nil)
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

	// Base event structure that all events will have
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
		// Create an order for this user
		order, err := s.createAndAddOrder(user)
		if err != nil {
			return models.EventMessage{}, fmt.Errorf("failed to create order: %w", err)
		}
		baseEvent.RestaurantID = order.RestaurantID

		eventData = struct {
			BaseEvent
			OrderID       string   `json:"orderId"`
			Items         []string `json:"item_ids"`
			TotalAmount   float64  `json:"totalAmount"`
			Status        string   `json:"status"`
			OrderPlacedAt int64    `json:"order_placed_at"`
		}{
			BaseEvent:     baseEvent,
			OrderID:       order.ID,
			Items:         order.Items,
			TotalAmount:   order.TotalAmount,
			Status:        order.Status,
			OrderPlacedAt: order.OrderPlacedAt.Unix(),
		}
		topic = "order_events"

	case models.EventPrepareOrder:
		order := event.Data.(*models.Order)
		baseEvent.RestaurantID = order.RestaurantID

		eventData = struct {
			BaseEvent
			OrderID       string `json:"orderId"`
			Status        string `json:"status"`
			PrepStartTime int64  `json:"prep_start_time"`
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
			PickupTime int64  `json:"pickup_time"`
		}{
			BaseEvent:  baseEvent,
			OrderID:    order.ID,
			Status:     order.Status,
			PickupTime: order.PickupTime.Unix(),
		}
		topic = "order_ready_events"

	// Add cases for other event types as needed, such as Delivery, Review, etc.
	default:
		return models.EventMessage{}, fmt.Errorf("unknown event type: %v", event.Type)
	}

	// Serialize the event to JSON
	data, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("Error serializing event: %v", err)
		return models.EventMessage{}, err
	}

	// Return the event message
	return models.EventMessage{
		Topic:   topic,
		Message: data,
	}, nil
}

// Event handlers
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

	// Update order status
	order.Status = models.OrderStatusPreparing

	// Estimate prep time
	prepTime := s.estimatePrepTime(restaurant, order.Items)

	// Add some variability to prep time
	variability := 0.2 // 20% variability
	actualPrepTime := prepTime * (1 + (rand.Float64()*2-1)*variability)

	// Calculate when the order will be ready
	readyTime := s.CurrentTime.Add(time.Duration(actualPrepTime) * time.Minute)

	// Update order
	order.PrepStartTime = s.CurrentTime
	order.PickupTime = readyTime

	// Update restaurant status
	restaurant.CurrentOrders = append(restaurant.CurrentOrders, *order)

	// Schedule the next event (order ready)
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
