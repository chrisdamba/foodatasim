package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/factories"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"math"
	"math/rand"
	"time"
)

type Simulator struct {
	*models.Simulator
	Config      models.Config
	CurrentTime time.Time
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

func (s *Simulator) getRestaurant(restaurantID string) *models.Restaurant {
	restaurant, exists := s.Restaurants[restaurantID]
	if !exists {
		return nil
	}
	return restaurant
}

func (s *Simulator) updateTrafficConditions() {
	for i := range s.TrafficConditions {
		s.TrafficConditions[i].Density = s.generateTrafficDensity(s.CurrentTime)
	}
}

func (s *Simulator) generateOrders() {
	for _, user := range s.Users {
		if s.shouldPlaceOrder(user) {
			order := s.createOrder(user)
			s.assignDeliveryPartner(order)
			s.Orders = append(s.Orders, order)
		}
	}
}

func (s *Simulator) updateOrderStatuses() {
	for i, order := range s.Orders {
		switch order.Status {
		case "placed":
			if s.CurrentTime.After(order.PrepStartTime) {
				s.Orders[i].Status = "preparing"
			}
		case "preparing":
			if s.CurrentTime.After(order.PickupTime) {
				s.Orders[i].Status = "ready_for_pickup"
			}
		case "ready_for_pickup":
			if s.isDeliveryPartnerAtRestaurant(order) {
				s.Orders[i].Status = "picked_up"
			}
		case "picked_up":
			if s.isOrderDelivered(order) {
				s.Orders[i].Status = "delivered"
				review := s.createReview(s.Orders[i])
				s.updateRatings(review)
			}
		}
	}
}

func (s *Simulator) generateReviews() {
	for _, order := range s.Orders {
		if order.Status == "delivered" && s.shouldGenerateReview(order) {
			review := s.createReview(order)
			s.Reviews = append(s.Reviews, review)
			s.updateRatings(review)
		}
	}
}

func (s *Simulator) shouldGenerateReview(order models.Order) bool {
	fake := faker.New()

	// Base probability of generating a review
	baseProbability := 0.3

	// Adjust probability based on order total amount
	// Higher value orders are more likely to receive a review
	if order.TotalAmount > 50 {
		baseProbability += 0.1
	} else if order.TotalAmount < 20 {
		baseProbability -= 0.1
	}

	// Adjust probability based on delivery time
	// Late deliveries are more likely to receive a review
	estimatedDeliveryTime := order.EstimatedDeliveryTime.Sub(order.OrderPlacedAt)
	actualDeliveryTime := order.ActualDeliveryTime.Sub(order.OrderPlacedAt)
	if actualDeliveryTime > estimatedDeliveryTime+(estimatedDeliveryTime/2) {
		baseProbability += 0.2
	} else if actualDeliveryTime < estimatedDeliveryTime {
		baseProbability += 0.1
	}

	// Adjust probability based on user's order frequency
	// Frequent users are more likely to leave reviews
	user := s.getUser(order.CustomerID)
	if user != nil {
		if user.OrderFrequency > 0.5 {
			baseProbability += 0.1
		} else if user.OrderFrequency < 0.1 {
			baseProbability -= 0.1
		}
	}

	// Ensure probability is within [0, 1] range
	baseProbability = math.Max(0, math.Min(1, baseProbability))

	// Generate a random number and compare with the calculated probability
	return fake.Float64(2, 0, 100)/100 < baseProbability
}

func (s *Simulator) createReview(order models.Order) models.Review {
	fake := faker.New()
	foodRating := generateRating()
	deliveryRating := generateRating()
	overallRating := (foodRating + deliveryRating) / 2
	return models.Review{
		ID:                generateID(),
		OrderID:           order.ID,
		CustomerID:        order.CustomerID,
		RestaurantID:      order.RestaurantID,
		DeliveryPartnerID: order.DeliveryPartnerID,
		FoodRating:        foodRating,
		DeliveryRating:    deliveryRating,
		OverallRating:     generateRating(),
		Comment:           generateComment(&fake, overallRating),
		CreatedAt:         s.CurrentTime,
		UpdatedAt:         s.CurrentTime,
		IsIgnored:         false,
	}
}

func (s *Simulator) updateRatings(review models.Review) {
	// Update restaurant rating
	restaurant := s.getRestaurant(review.RestaurantID)
	restaurant.Rating = updateRating(restaurant.Rating, review.FoodRating, s.Config.RestaurantRatingAlpha)
	restaurant.TotalRatings++

	// Update delivery partner rating
	partner := s.getDeliveryPartner(review.DeliveryPartnerID)
	partner.Rating = updateRating(partner.Rating, review.DeliveryRating, s.Config.PartnerRatingAlpha)
	partner.TotalRatings++
}

func updateRating(currentRating, newRating, alpha float64) float64 {
	updatedRating := (alpha * newRating) + ((1 - alpha) * currentRating)
	return math.Max(1, math.Min(5, updatedRating))
}

func generateRating() float64 {
	// Generate a rating between 1 and 5, with a bias towards higher ratings
	x := rand.Float64()
	return 1.0 + 4.0*math.Pow(x, 2)
}

func generateComment(fake *faker.Faker, rating float64) string {
	// Define comment templates based on rating ranges
	excellentComments := []string{
		"Absolutely delicious! The food was outstanding.",
		"Fantastic service and the meal was top-notch.",
		"Best order I've had in a while. Highly recommend!",
		"Speedy delivery and the food was perfect.",
		"Five stars! The meal exceeded my expectations.",
	}

	goodComments := []string{
		"Enjoyed the food. Delivery was on time.",
		"Good meal and friendly service.",
		"Solid choice. Will order again.",
		"Pretty good food. No complaints.",
		"The order was tasty. Delivery was quick.",
	}

	averageComments := []string{
		"The food was okay. Nothing special.",
		"Decent meal, but delivery took longer than expected.",
		"Average experience. Food could be better.",
		"The order was alright. Might try something else next time.",
		"Not bad, not great. Just okay.",
	}

	poorComments := []string{
		"Disappointed with the food. Not as described.",
		"The meal was cold when it arrived.",
		"Long wait for mediocre food.",
		"Wouldn't recommend. Below average.",
		"Poor quality. Expected better.",
	}

	// Select comment template based on rating
	var comment string
	switch {
	case rating >= 4.5:
		comment = fake.RandomStringElement(excellentComments)
	case rating >= 3.5:
		comment = fake.RandomStringElement(goodComments)
	case rating >= 2.5:
		comment = fake.RandomStringElement(averageComments)
	default:
		comment = fake.RandomStringElement(poorComments)
	}

	// Optionally add an adjective
	if fake.Bool() {
		adjectives := []string{"delicious", "tasty", "flavorful", "mouthwatering", "satisfying", "disappointing", "bland", "mediocre"}
		comment = fake.RandomStringElement(adjectives) + " " + comment
	}

	// Optionally add an emoji
	if fake.Bool() {
		emojis := []string{"😋", "👍", "🍽️", "🌟", "😊", "🍕", "🍔", "🍜", "🍣", "🍱"}
		comment += " " + fake.RandomStringElement(emojis)
	}

	return comment
}

func (s *Simulator) updateDeliveryPartnerLocations() {
	for i, partner := range s.DeliveryPartners {
		switch partner.Status {
		case "available":
			s.DeliveryPartners[i].CurrentLocation = s.moveTowardsHotspot(partner)
		case "en_route_to_pickup":
			order := s.getPartnerCurrentOrder(partner)
			restaurant := s.getRestaurant(order.RestaurantID)
			s.DeliveryPartners[i].CurrentLocation = s.moveTowards(partner.CurrentLocation, restaurant.Location)
			if s.isAtLocation(partner.CurrentLocation, restaurant.Location) {
				s.DeliveryPartners[i].Status = "waiting_for_pickup"
			}
		case "en_route_to_delivery":
			order := s.getPartnerCurrentOrder(partner)
			user := s.getUser(order.CustomerID)
			s.DeliveryPartners[i].CurrentLocation = s.moveTowards(partner.CurrentLocation, user.Location)
		}
	}
}

func (s *Simulator) updateUserBehavior() {
	for i, user := range s.Users {
		s.Users[i].OrderFrequency = s.adjustOrderFrequency(user)
	}
}

func (s *Simulator) updateRestaurantStatus() {
	for i, restaurant := range s.Restaurants {
		s.Restaurants[i].PrepTime = s.adjustPrepTime(restaurant)
		s.Restaurants[i].PickupEfficiency = s.adjustPickupEfficiency(restaurant)
	}
}
