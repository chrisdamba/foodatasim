package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

const earthRadiusKm = 6371.0 // Earth's radius in kilometers

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

func (s *Simulator) selectRestaurant(user models.User) *models.Restaurant {
	// Get restaurants within a certain radius of the user
	nearbyRestaurants := s.getNearbyRestaurants(user.Location, 5.0) // 5.0 km radius

	if len(nearbyRestaurants) == 0 {
		// If no restaurants nearby, expand the search radius
		nearbyRestaurants = s.getNearbyRestaurants(user.Location, 10.0)
	}

	// If still no restaurants, return a random restaurant (fallback)
	if len(nearbyRestaurants) == 0 {
		keys := make([]string, 0, len(s.Restaurants))
		for k := range s.Restaurants {
			keys = append(keys, k)
		}
		return s.Restaurants[keys[rand.Intn(len(keys))]]
	}

	// Calculate scores for each nearby restaurant
	type restaurantScore struct {
		restaurant *models.Restaurant
		score      float64
	}

	scoredRestaurants := make([]restaurantScore, len(nearbyRestaurants))

	for i, restaurant := range nearbyRestaurants {
		score := s.calculateRestaurantScore(restaurant, user)
		scoredRestaurants[i] = restaurantScore{restaurant, score}
	}

	// Sort restaurants by score in descending order
	sort.Slice(scoredRestaurants, func(i, j int) bool {
		return scoredRestaurants[i].score > scoredRestaurants[j].score
	})

	// Select a restaurant probabilistically based on scores
	totalScore := 0.0
	for _, rs := range scoredRestaurants {
		totalScore += rs.score
	}

	randomValue := rand.Float64() * totalScore
	cumulativeScore := 0.0

	for _, rs := range scoredRestaurants {
		cumulativeScore += rs.score
		if randomValue <= cumulativeScore {
			return rs.restaurant
		}
	}

	// Fallback: return the highest scored restaurant
	return scoredRestaurants[0].restaurant
}

func (s *Simulator) getNearbyRestaurants(userLocation models.Location, radius float64) []*models.Restaurant {
	var nearbyRestaurants []*models.Restaurant
	for _, restaurant := range s.Restaurants {
		if distance := s.calculateDistance(userLocation, restaurant.Location); distance <= radius {
			nearbyRestaurants = append(nearbyRestaurants, restaurant)
		}
	}
	return nearbyRestaurants
}

func (s *Simulator) calculateRestaurantScore(restaurant *models.Restaurant, user models.User) float64 {
	// Base score is the restaurant's rating
	score := restaurant.Rating

	// Adjust score based on user preferences
	for _, cuisine := range restaurant.Cuisines {
		if contains(user.Preferences, cuisine) {
			score += 1.0
		}
	}

	// Adjust score based on distance (closer is better)
	distance := s.calculateDistance(user.Location, restaurant.Location)
	score += 5.0 / (1.0 + distance) // This will add between 0 and 5 to the score, with closer restaurants getting a higher boost

	// Adjust score based on time of day (e.g., breakfast places in the morning)
	if isBreakfastTime(s.CurrentTime) && contains(restaurant.Cuisines, "Breakfast") {
		score += 2.0
	}

	// Adjust score based on restaurant's recent order volume (popularity boost)
	recentOrderCount := s.getRecentOrderCount(restaurant.ID)
	score += float64(recentOrderCount) * 0.1 // Small boost for each recent order

	return score
}

func (s *Simulator) calculateDistance(loc1, loc2 models.Location) float64 {
	// Convert latitude and longitude from degrees to radians
	lat1 := degreesToRadians(loc1.Lat)
	lon1 := degreesToRadians(loc1.Lon)
	lat2 := degreesToRadians(loc2.Lat)
	lon2 := degreesToRadians(loc2.Lon)

	// Haversine formula
	dlat := lat2 - lat1
	dlon := lon2 - lon1
	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// Calculate the distance
	distance := earthRadiusKm * c

	return distance // Returns distance in kilometers
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isBreakfastTime(t time.Time) bool {
	hour := t.Hour()
	return hour >= 6 && hour < 11
}

func (s *Simulator) getRecentOrderCount(restaurantID string) int {
	// Count orders for this restaurant in the last 24 hours
	count := 0
	for _, order := range s.Orders {
		if order.RestaurantID == restaurantID && s.CurrentTime.Sub(order.OrderPlacedAt) <= 24*time.Hour {
			count++
		}
	}
	return count
}

func (s *Simulator) generateTrafficDensity(t time.Time) float64 {
	baseTraffic := 0.5 + 0.5*math.Sin(float64(t.Hour())/24*2*math.Pi)
	randomFactor := 1 + (rand.Float64()-0.5)*s.Config.TrafficVariability
	return baseTraffic * randomFactor
}

func (s *Simulator) shouldPlaceOrder(user models.User) bool {
	hourFactor := 1.0
	if s.isPeakHour(s.CurrentTime) {
		hourFactor = s.Config.PeakHourFactor
	}
	if s.isWeekend(s.CurrentTime) {
		hourFactor *= s.Config.WeekendFactor
	}

	orderProbability := user.OrderFrequency * hourFactor / (24 * 60) // Convert to per-minute probability
	return rand.Float64() < orderProbability
}

func (s *Simulator) createOrder(user models.User) models.Order {
	restaurant := s.selectRestaurant(user)
	items := s.selectMenuItems(restaurant, user)
	totalAmount := s.calculateTotalAmount(items)
	prepTime := s.estimatePrepTime(restaurant, items)

	order := models.Order{
		ID:            generateID(),
		CustomerID:    user.ID,
		RestaurantID:  restaurant.ID,
		Items:         items,
		TotalAmount:   totalAmount,
		OrderPlacedAt: s.CurrentTime,
		PrepStartTime: s.CurrentTime.Add(time.Minute * time.Duration(rand.Intn(5))),
		Status:        "placed",
	}

	order.PickupTime = order.PrepStartTime.Add(time.Minute * time.Duration(prepTime))
	return order
}

func (s *Simulator) updateOrderStatus(orderID string, status string) {
	for i, order := range s.Orders {
		if order.ID == orderID {
			s.Orders[i].Status = status
			if status == "delivered" {
				s.CompletedOrdersByRestaurant[order.RestaurantID] = append(s.CompletedOrdersByRestaurant[order.RestaurantID], s.Orders[i])
			}
			break
		}
	}
}

func (s *Simulator) addOrder(order models.Order) {
	s.Orders = append(s.Orders, order)
	s.OrdersByUser[order.CustomerID] = append(s.OrdersByUser[order.CustomerID], order)
}

func (s *Simulator) getPartnerCurrentOrder(partner models.DeliveryPartner) *models.Order {
	for i := len(s.Orders) - 1; i >= 0; i-- {
		order := &s.Orders[i]
		if order.DeliveryPartnerID == partner.ID &&
			(order.Status == "picked_up" || order.Status == "ready_for_pickup") {
			return order
		}
	}
	return nil
}

func (s *Simulator) getUser(userID string) *models.User {
	for i, user := range s.Users {
		if user.ID == userID {
			return &s.Users[i]
		}
	}
	return nil
}

func (s *Simulator) getDeliveryPartner(partnerID string) *models.DeliveryPartner {
	for i, partner := range s.DeliveryPartners {
		if partner.ID == partnerID {
			return &s.DeliveryPartners[i]
		}
	}
	return nil
}

func (s *Simulator) isDeliveryPartnerAtRestaurant(order models.Order) bool {
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	restaurant := s.getRestaurant(order.RestaurantID)
	if partner == nil || restaurant == nil {
		return false
	}
	return s.isAtLocation(partner.CurrentLocation, restaurant.Location)
}

func (s *Simulator) isOrderDelivered(order models.Order) bool {
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	user := s.getUser(order.CustomerID)
	if partner == nil || user == nil {
		return false
	}
	return s.isAtLocation(partner.CurrentLocation, user.Location)
}

func (s *Simulator) addMenuItemToRestaurant(restaurantID string, menuItem *models.MenuItem) {
	restaurant := s.Restaurants[restaurantID]
	restaurant.MenuItems = append(restaurant.MenuItems, menuItem.ID)
	s.MenuItems[menuItem.ID] = menuItem
}

func (s *Simulator) selectMenuItems(restaurant *models.Restaurant, user models.User) []string {
	// Define meal types
	// mealTypes := []string{"appetizer", "main course", "side dish", "dessert", "drink"}

	// Decide on the meal composition
	var mealComposition []string
	if rand.Float32() < 0.7 { // 70% chance of a full meal
		mealComposition = []string{"main course", "side dish", "drink"}
		if rand.Float32() < 0.3 { // 30% chance to add an appetizer
			mealComposition = append(mealComposition, "appetizer")
		}
		if rand.Float32() < 0.2 { // 20% chance to add a dessert
			mealComposition = append(mealComposition, "dessert")
		}
	} else { // 30% chance of a simpler order
		mealComposition = []string{"main course", "drink"}
	}

	selectedItems := make([]string, 0, len(mealComposition))

	for _, mealType := range mealComposition {
		// Filter items by meal type
		var eligibleItems []*models.MenuItem
		for _, itemID := range restaurant.MenuItems {
			item := s.getMenuItem(itemID)
			if item.Type == mealType {
				eligibleItems = append(eligibleItems, item)
			}
		}

		if len(eligibleItems) == 0 {
			continue // Skip if no items of this type
		}

		// Calculate selection probabilities based on popularity and user preferences
		probabilities := make([]float64, len(eligibleItems))
		totalProb := 0.0

		for i, item := range eligibleItems {
			prob := item.Popularity

			// Consider user preferences (assuming User struct has Preferences field)
			for _, pref := range user.Preferences {
				if strings.Contains(strings.ToLower(item.Name), strings.ToLower(pref)) {
					prob *= 1.5 // Increase probability for preferred items
				}
			}

			// Consider dietary restrictions (assuming User struct has DietaryRestrictions field)
			if !s.hasConflictingIngredients(item, user.DietaryRestrictions) {
				probabilities[i] = prob
				totalProb += prob
			}
		}

		// Select an item based on calculated probabilities
		if totalProb > 0 {
			randValue := rand.Float64() * totalProb
			cumulativeProb := 0.0
			for i, prob := range probabilities {
				cumulativeProb += prob
				if randValue <= cumulativeProb {
					selectedItems = append(selectedItems, eligibleItems[i].ID)
					break
				}
			}
		}
	}

	return selectedItems
}

func (s *Simulator) getMenuItem(itemID string) *models.MenuItem {
	item, exists := s.MenuItems[itemID]
	if !exists {
		return nil // Return nil if the item doesn't exist
	}
	return item
}

func (s *Simulator) hasConflictingIngredients(item *models.MenuItem, restrictions []string) bool {
	for _, ingredient := range item.Ingredients {
		for _, restriction := range restrictions {
			if strings.EqualFold(ingredient, restriction) {
				return true
			}
		}
	}
	return false
}

func (s *Simulator) calculateTotalAmount(items []string) float64 {
	var subtotal float64
	var discountableTotal float64
	var discountApplied bool
	var discountAmount float64

	for _, itemID := range items {
		item := s.getMenuItem(itemID)
		if item == nil {
			continue // Skip if item not found
		}

		subtotal += item.Price

		// Add to discountable total if the item is eligible for discounts
		if item.IsDiscountEligible {
			discountableTotal += item.Price
		}
	}

	// Apply discount if applicable
	if discountableTotal >= s.Config.MinOrderForDiscount && !discountApplied {
		discountAmount = discountableTotal * s.Config.DiscountPercentage
		if discountAmount > s.Config.MaxDiscountAmount {
			discountAmount = s.Config.MaxDiscountAmount
		}
		discountApplied = true
	}

	// Calculate tax
	taxAmount := subtotal * s.Config.TaxRate

	// Calculate delivery fee (if applicable)
	deliveryFee := s.calculateDeliveryFee(subtotal)

	// Calculate service fee
	serviceFee := subtotal * s.Config.ServiceFeePercentage

	// Calculate total
	total := subtotal + taxAmount + deliveryFee + serviceFee - discountAmount

	// Round to two decimal places
	return math.Round(total*100) / 100
}

func (s *Simulator) calculateDeliveryFee(subtotal float64) float64 {
	if subtotal >= s.Config.FreeDeliveryThreshold {
		return 0
	}

	// Base delivery fee
	fee := s.Config.BaseDeliveryFee

	// Additional fee for small orders
	if subtotal < s.Config.SmallOrderThreshold {
		fee += s.Config.SmallOrderFee
	}

	return fee
}

func (s *Simulator) estimatePrepTime(restaurant *models.Restaurant, items []string) float64 {
	baseTime := restaurant.AvgPrepTime
	totalComplexity := 0.0

	for _, itemID := range items {
		item := s.getMenuItem(itemID)
		if item != nil {
			totalComplexity += item.PrepComplexity
		}
	}

	// Adjust prep time based on order complexity
	adjustedTime := baseTime * (1 + (totalComplexity/float64(len(items))-1)*0.2)

	// Consider restaurant's current load
	currentLoad := float64(len(restaurant.CurrentOrders)) / float64(restaurant.Capacity)
	loadFactor := 1 + (currentLoad * 0.5) // Up to 50% increase for full capacity

	// Add some randomness to account for unforeseen factors
	randomFactor := 1 + (rand.Float64()-0.5)*0.1 // ±5% random variation

	finalPrepTime := adjustedTime * loadFactor * randomFactor

	return math.Max(finalPrepTime, restaurant.MinPrepTime)
}

func (s *Simulator) getAvailablePartnersNear(location models.Location) []models.DeliveryPartner {
	availablePartners := make([]models.DeliveryPartner, 0)
	for _, partner := range s.DeliveryPartners {
		if partner.Status == "available" && s.isNearLocation(partner.CurrentLocation, location) {
			availablePartners = append(availablePartners, partner)
		}
	}
	return availablePartners
}

func (s *Simulator) isNearLocation(loc1, loc2 models.Location) bool {
	distance := s.calculateDistance(loc1, loc2)

	// Base threshold
	threshold := s.Config.NearLocationThreshold

	// Adjust threshold based on time of day (e.g., wider range during off-peak hours)
	if !s.isPeakHour(s.CurrentTime) {
		threshold *= 1.5
	}

	// Adjust threshold based on urban density
	if s.isUrbanArea(loc1) && s.isUrbanArea(loc2) {
		threshold *= 0.8 // Smaller threshold in urban areas
	}

	return distance <= threshold
}

func (s *Simulator) isUrbanArea(loc models.Location) bool {
	// Implement logic to determine if a location is in an urban area
	// This could be based on population density data or predefined urban zones
	// For simplicity, let's assume a central urban area:
	cityCenter := models.Location{Lat: s.Config.CityLat, Lon: s.Config.CityLon}
	return s.calculateDistance(loc, cityCenter) <= s.Config.UrbanRadius
}

func (s *Simulator) getPartnerIndex(partnerID string) int {
	for i, partner := range s.DeliveryPartners {
		if partner.ID == partnerID {
			return i
		}
	}
	return -1 // Return -1 if partner not found
}

func (s *Simulator) assignDeliveryPartner(order models.Order) {
	restaurant := s.getRestaurant(order.RestaurantID)
	availablePartners := s.getAvailablePartnersNear(restaurant.Location)
	if len(availablePartners) > 0 {
		partner := availablePartners[rand.Intn(len(availablePartners))]
		order.DeliveryPartnerID = partner.ID
		partnerIndex := s.getPartnerIndex(partner.ID)
		s.DeliveryPartners[partnerIndex].Status = "en_route_to_pickup"
	}
}

func (s *Simulator) moveTowardsHotspot(partner models.DeliveryPartner) models.Location {
	// Find the nearest hotspot
	nearestHotspot := s.findNearestHotspot(partner.CurrentLocation)

	// Move towards the hotspot
	return s.moveTowards(partner.CurrentLocation, nearestHotspot)
}

func (s *Simulator) findNearestHotspot(loc models.Location) models.Location {
	// Define a list of hotspots
	hotspots := []models.Hotspot{
		{Location: models.Location{Lat: s.Config.CityLat, Lon: s.Config.CityLon}, Weight: 1.0},                 // City center
		{Location: models.Location{Lat: s.Config.CityLat + 0.01, Lon: s.Config.CityLon + 0.01}, Weight: 0.8},   // Business district
		{Location: models.Location{Lat: s.Config.CityLat - 0.015, Lon: s.Config.CityLon - 0.005}, Weight: 0.7}, // University area
		{Location: models.Location{Lat: s.Config.CityLat + 0.008, Lon: s.Config.CityLon - 0.012}, Weight: 0.6}, // Shopping mall
		{Location: models.Location{Lat: s.Config.CityLat - 0.02, Lon: s.Config.CityLon + 0.018}, Weight: 0.5},  // Residential area
	}

	var nearestHotspot models.Hotspot
	minDistance := math.Inf(1)

	for _, hotspot := range hotspots {
		distance := s.calculateDistance(loc, hotspot.Location)

		// Adjust distance by hotspot weight (more important hotspots seem "closer")
		adjustedDistance := distance / hotspot.Weight

		if adjustedDistance < minDistance {
			minDistance = adjustedDistance
			nearestHotspot = hotspot
		}
	}

	// Add some randomness to the chosen hotspot
	jitter := 0.001 // About 111 meters
	nearestHotspot.Location.Lat += (rand.Float64() - 0.5) * jitter
	nearestHotspot.Location.Lon += (rand.Float64() - 0.5) * jitter

	return nearestHotspot.Location
}

func (s *Simulator) moveTowards(from, to models.Location) models.Location {
	distance := s.calculateDistance(from, to)
	if distance <= s.Config.PartnerMoveSpeed {
		return to // If we can reach the destination, go directly there
	}

	// Calculate the ratio of how far we can move
	ratio := s.Config.PartnerMoveSpeed / distance

	// Calculate new position
	newLat := from.Lat + (to.Lat-from.Lat)*ratio
	newLon := from.Lon + (to.Lon-from.Lon)*ratio

	return models.Location{Lat: newLat, Lon: newLon}
}

func (s *Simulator) isAtLocation(loc1, loc2 models.Location) bool {
	return math.Abs(loc1.Lat-loc2.Lat) < s.Config.LocationPrecision &&
		math.Abs(loc1.Lon-loc2.Lon) < s.Config.LocationPrecision
}

func (s *Simulator) adjustOrderFrequency(user models.User) float64 {
	recentOrders := s.getRecentOrders(user.ID, s.Config.UserBehaviorWindow)
	if len(recentOrders) == 0 {
		return user.OrderFrequency // No recent orders, no change
	}

	// Calculate time between orders
	var totalTimeBetween float64
	for i := 1; i < len(recentOrders); i++ {
		timeBetween := recentOrders[i].OrderPlacedAt.Sub(recentOrders[i-1].OrderPlacedAt).Hours()
		totalTimeBetween += timeBetween
	}
	avgTimeBetween := totalTimeBetween / float64(len(recentOrders)-1)

	// Convert to frequency (orders per day)
	newFrequency := 24 / avgTimeBetween

	// Gradually adjust towards new frequency
	adjustmentRate := 0.2 // 20% adjustment towards new frequency
	return user.OrderFrequency + (newFrequency-user.OrderFrequency)*adjustmentRate
}

func (s *Simulator) adjustPrepTime(restaurant *models.Restaurant) float64 {
	currentLoad := float64(len(restaurant.CurrentOrders)) / float64(restaurant.Capacity)
	loadFactor := 1 + (currentLoad * s.Config.RestaurantLoadFactor)

	// Adjust prep time based on current load
	adjustedPrepTime := restaurant.AvgPrepTime * loadFactor

	// Ensure prep time doesn't go below minimum
	return math.Max(adjustedPrepTime, restaurant.MinPrepTime)
}

func (s *Simulator) adjustPickupEfficiency(restaurant *models.Restaurant) float64 {
	recentOrders := s.getRecentCompletedOrders(restaurant.ID, 20) // Consider last 20 orders
	if len(recentOrders) == 0 {
		return restaurant.PickupEfficiency // No recent orders, no change
	}

	var totalEfficiency float64
	for _, order := range recentOrders {
		actualPrepTime := order.PickupTime.Sub(order.PrepStartTime).Minutes()
		efficiency := restaurant.AvgPrepTime / actualPrepTime
		totalEfficiency += efficiency
	}
	avgEfficiency := totalEfficiency / float64(len(recentOrders))

	// Gradually adjust towards new efficiency
	return restaurant.PickupEfficiency + (avgEfficiency-restaurant.PickupEfficiency)*s.Config.EfficiencyAdjustRate
}

func (s *Simulator) getRecentOrders(userID string, count int) []models.Order {
	var recentOrders []models.Order

	// Iterate through orders in reverse (assuming orders are stored chronologically)
	for i := len(s.Orders) - 1; i >= 0 && len(recentOrders) < count; i-- {
		if s.Orders[i].CustomerID == userID {
			recentOrders = append(recentOrders, s.Orders[i])
		}
	}

	return recentOrders
}

func (s *Simulator) getRecentCompletedOrders(restaurantID string, count int) []models.Order {
	var recentCompletedOrders []models.Order

	// Iterate through orders in reverse (assuming orders are stored chronologically)
	for i := len(s.Orders) - 1; i >= 0 && len(recentCompletedOrders) < count; i-- {
		if s.Orders[i].RestaurantID == restaurantID && s.Orders[i].Status == "delivered" {
			recentCompletedOrders = append(recentCompletedOrders, s.Orders[i])
		}
	}

	return recentCompletedOrders
}

func (s *Simulator) isPeakHour(t time.Time) bool {
	hour := t.Hour()
	return (hour >= 11 && hour <= 14) || (hour >= 18 && hour <= 21)
}

func (s *Simulator) isWeekend(t time.Time) bool {
	day := t.Weekday()
	return day == time.Saturday || day == time.Sunday
}

func (s *Simulator) initializeTrafficConditions() {
	// Initialize traffic conditions for different times of the day
	for hour := 0; hour < 24; hour++ {
		trafficTime := s.Config.StartDate.Add(time.Duration(hour) * time.Hour)
		s.TrafficConditions = append(s.TrafficConditions, models.TrafficCondition{
			Time: trafficTime,
			Location: models.Location{
				Lat: s.Config.CityLat,
				Lon: s.Config.CityLon,
			},
			Density: generateTrafficDensity(hour),
		})
	}
}

func generateTrafficDensity(hour int) float64 {
	fake := faker.New()
	switch {
	case hour >= 7 && hour <= 9, hour >= 16 && hour <= 18:
		return fake.Float64(2, 70, 100) / 100
	case hour >= 22 || hour <= 5:
		return fake.Float64(2, 0, 30) / 100
	default:
		return fake.Float64(2, 30, 70) / 100
	}
}

func generateID() string {
	return cuid.New()
}
