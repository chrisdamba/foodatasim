package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/factories"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"time"
)

type Simulator struct {
	*models.Simulator
	Config      models.Config
	CurrentTime time.Time
	EventQueue  *models.EventQueue
}

func NewSimulator(config models.Config) *Simulator {
	sim := &Simulator{
		Simulator: &models.Simulator{
			Config:      config,
			CurrentTime: config.StartDate,
			Restaurants: make(map[string]*models.Restaurant),
			MenuItems:   make(map[string]*models.MenuItem),
		},
	}
	sim.initializeData()
	return sim
}

func (s *Simulator) initializeData() {
	userFactory := &factories.UserFactory{}
	restaurantFactory := &factories.RestaurantFactory{}
	menuItemFactory := &factories.MenuItemFactory{}
	deliveryPartnerFactory := &factories.DeliveryPartnerFactory{}

	// initialise users
	s.Users = make([]models.User, s.Config.InitialUsers)
	for i := 0; i < s.Config.InitialUsers; i++ {
		s.Users[i] = userFactory.CreateUser(s.Config)
	}

	// Initialize restaurants
	s.Restaurants = make(map[string]*models.Restaurant)
	for i := 0; i < s.Config.InitialRestaurants; i++ {
		restaurant := restaurantFactory.CreateRestaurant(s.Config)
		s.Restaurants[restaurant.ID] = &restaurant
	}

	// Initialize delivery partners
	s.DeliveryPartners = make([]models.DeliveryPartner, s.Config.InitialPartners)
	for i := 0; i < s.Config.InitialPartners; i++ {
		s.DeliveryPartners[i] = deliveryPartnerFactory.CreateDeliveryPartner(s.Config)
	}

	// Initialize menu items
	fake := faker.New()
	s.MenuItems = make(map[string]*models.MenuItem)
	for _, restaurant := range s.Restaurants {
		itemCount := fake.IntBetween(10, 30)
		for i := 0; i < itemCount; i++ {
			menuItem := menuItemFactory.CreateMenuItem(restaurant)
			s.MenuItems[menuItem.ID] = &menuItem
			restaurant.MenuItems = append(restaurant.MenuItems, menuItem.ID)
		}
	}

	// Initialize traffic conditions
	s.initializeTrafficConditions()

	// Initialize maps
	s.OrdersByUser = make(map[string][]models.Order)
	s.CompletedOrdersByRestaurant = make(map[string][]models.Order)
	s.EventQueue = models.NewEventQueue()
}

func (s *Simulator) Run() {
	for s.CurrentTime.Before(s.Config.EndDate) {
		s.simulateTimeStep()
		s.CurrentTime = s.CurrentTime.Add(time.Minute) // Simulate in 1-minute increments
	}
}

func (s *Simulator) simulateTimeStep() {
	s.updateTrafficConditions()
	s.generateOrders()
	s.updateOrderStatuses()
	s.updateDeliveryPartnerLocations()
	s.updateUserBehavior()
	s.updateRestaurantStatus()
}
