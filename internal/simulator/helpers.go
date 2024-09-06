package simulator

import (
	"encoding/csv"
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const earthRadiusKm = 6371.0 // Earth's radius in kilometers

func (s *Simulator) getUser(userID string) *models.User {
	for i, user := range s.Users {
		if user.ID == userID {
			return s.Users[i]
		}
	}
	return nil
}

func (s *Simulator) updateUserBehaviour() {
	for i, user := range s.Users {
		orderFrequency := s.adjustOrderFrequency(user)
		s.EventQueue.Enqueue(&models.Event{
			Time: s.CurrentTime,
			Type: models.EventUpdateUserBehaviour,
			Data: &models.UserBehaviourUpdate{
				UserID:         user.ID,
				OrderFrequency: orderFrequency,
			},
		})
		s.Users[i].OrderFrequency = orderFrequency
	}
}

func (s *Simulator) generateReviews() {
	for _, order := range s.Orders {
		if order.Status == "delivered" && s.shouldGenerateReview(&order) {
			review := s.createReview(&order)
			s.Reviews = append(s.Reviews, review)
			s.updateRatings(review)
		}
	}
}

func (s *Simulator) shouldGenerateReview(order *models.Order) bool {
	// if a review has already been generated for this order, don't generate another
	if order.ReviewGenerated {
		return false
	}

	// base probability of generating a review
	baseProbability := 0.3

	// adjust probability based on order total amount
	// higher value orders are more likely to receive a review
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
	return s.Rng.Float64() < baseProbability
}

func (s *Simulator) createReview(order *models.Order) models.Review {
	// select a random review from our data for the food rating and comment
	reviewData := s.Config.ReviewData[s.Rng.Intn(len(s.Config.ReviewData))]

	// generate food rating based on whether the review was liked or not
	var foodRating float64
	if reviewData.Liked {
		foodRating = 3 + s.Rng.Float64()*2 // Random rating between 3 and 5
	} else {
		foodRating = 1 + s.Rng.Float64()*2 // Random rating between 1 and 3
	}

	// calculate delivery rating based on delivery performance
	deliveryRating := s.calculateDeliveryRating(order)

	// calculate overall rating
	overallRating := (foodRating + deliveryRating) / 2

	// adjust the comment to include delivery feedback
	comment := s.adjustCommentWithDeliveryFeedback(reviewData.Comment, deliveryRating)

	return models.Review{
		ID:                generateID(),
		OrderID:           order.ID,
		CustomerID:        order.CustomerID,
		RestaurantID:      order.RestaurantID,
		DeliveryPartnerID: order.DeliveryPartnerID,
		FoodRating:        foodRating,
		DeliveryRating:    deliveryRating,
		OverallRating:     overallRating,
		Comment:           comment,
		CreatedAt:         s.CurrentTime,
		UpdatedAt:         s.CurrentTime,
		IsIgnored:         false,
	}
}

func (s *Simulator) updateRatings(review models.Review) {
	// update restaurant rating
	restaurant := s.getRestaurant(review.RestaurantID)
	restaurant.Rating = updateRating(restaurant.Rating, review.FoodRating, s.Config.RestaurantRatingAlpha)
	restaurant.TotalRatings++

	// update delivery partner rating
	partner := s.getDeliveryPartner(review.DeliveryPartnerID)
	partner.Rating = updateRating(partner.Rating, review.DeliveryRating, s.Config.PartnerRatingAlpha)
	partner.TotalRatings++
}

func (s *Simulator) calculateDeliveryRating(order *models.Order) float64 {
	estimatedDeliveryTime := order.EstimatedDeliveryTime.Sub(order.OrderPlacedAt)
	actualDeliveryTime := order.ActualDeliveryTime.Sub(order.OrderPlacedAt)

	// calculate the difference between actual and estimated delivery time
	timeDifference := actualDeliveryTime - estimatedDeliveryTime

	// convert time difference to minutes
	minutesDifference := timeDifference.Minutes()

	// calculate base rating
	var baseRating float64
	switch {
	case minutesDifference <= -10: // More than 10 minutes early
		baseRating = 5.0
	case minutesDifference <= 0: // Up to 10 minutes early
		baseRating = 4.5
	case minutesDifference <= 10: // Up to 10 minutes late
		baseRating = 4.0
	case minutesDifference <= 20: // 10-20 minutes late
		baseRating = 3.0
	case minutesDifference <= 30: // 20-30 minutes late
		baseRating = 2.0
	default: // More than 30 minutes late
		baseRating = 1.0
	}

	// add some randomness (±0.5 stars)
	rating := baseRating + (s.Rng.Float64() - 0.5)

	// ensure rating is between 1 and 5
	return math.Max(1, math.Min(5, rating))
}

func (s *Simulator) adjustCommentWithDeliveryFeedback(originalComment string, deliveryRating float64) string {
	deliveryComments := []string{
		"Delivery was lightning fast! ",
		"Arrived earlier than expected. ",
		"Delivery was on time. ",
		"Delivery was a bit slow. ",
		"The wait for delivery was too long. ",
		"Extremely slow delivery. ",
	}

	var deliveryComment string
	switch {
	case deliveryRating >= 4.5:
		deliveryComment = deliveryComments[0]
	case deliveryRating >= 4.0:
		deliveryComment = deliveryComments[1]
	case deliveryRating >= 3.5:
		deliveryComment = deliveryComments[2]
	case deliveryRating >= 2.5:
		deliveryComment = deliveryComments[3]
	case deliveryRating >= 1.5:
		deliveryComment = deliveryComments[4]
	default:
		deliveryComment = deliveryComments[5]
	}

	// randomly decide whether to prepend or append the delivery comment
	if s.Rng.Float64() < 0.5 {
		return deliveryComment + originalComment
	} else {
		return originalComment + " " + deliveryComment
	}
}

func (s *Simulator) updateRestaurantStatus() {
	for i, restaurant := range s.Restaurants {
		s.Restaurants[i].PrepTime = s.adjustPrepTime(restaurant)
		s.Restaurants[i].PickupEfficiency = s.adjustPickupEfficiency(restaurant)
		s.EventQueue.Enqueue(&models.Event{
			Time: s.CurrentTime,
			Type: models.EventUpdateRestaurantStatus,
			Data: s.Restaurants[i],
		})
	}
}

func (s *Simulator) getRestaurant(restaurantID string) *models.Restaurant {
	restaurant, exists := s.Restaurants[restaurantID]
	if !exists {
		return nil
	}
	return restaurant
}

func (s *Simulator) selectRestaurant(user *models.User) *models.Restaurant {
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
		return s.Restaurants[keys[s.Rng.Intn(len(keys))]]
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

	randomValue := s.Rng.Float64() * totalScore
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

func (s *Simulator) calculateRestaurantScore(restaurant *models.Restaurant, user *models.User) float64 {
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

func (s *Simulator) updateTrafficConditions() {
	for i := range s.TrafficConditions {
		s.TrafficConditions[i].Density = s.generateTrafficDensity(s.CurrentTime)
	}
}

func (s *Simulator) generateTrafficDensity(t time.Time) float64 {
	baseTraffic := 0.5 + 0.5*math.Sin(float64(t.Hour())/24*2*math.Pi)
	randomFactor := 1 + (s.Rng.Float64()-0.5)*s.Config.TrafficVariability
	return baseTraffic * randomFactor
}

func (s *Simulator) generateOrders() {
	for _, user := range s.Users {
		if s.shouldPlaceOrder(user) {
			order := s.createOrder(user)
			s.assignDeliveryPartner(order)
			s.Orders = append(s.Orders, *order)
			s.EventQueue.Enqueue(&models.Event{
				Time: s.CurrentTime,
				Type: models.EventPlaceOrder,
				Data: user,
			})
		}
	}
}

func (s *Simulator) addOrder(order models.Order) {
	s.Orders = append(s.Orders, order)
	s.OrdersByUser[order.CustomerID] = append(s.OrdersByUser[order.CustomerID], order)
}

func (s *Simulator) createOrder(user *models.User) *models.Order {
	restaurant := s.selectRestaurant(user)
	items := s.selectMenuItems(restaurant, user)
	totalAmount := s.calculateTotalAmount(items)
	prepTime := s.estimatePrepTime(restaurant, items)

	order := &models.Order{
		ID:            generateID(),
		CustomerID:    user.ID,
		RestaurantID:  restaurant.ID,
		Items:         items,
		TotalAmount:   totalAmount,
		OrderPlacedAt: s.CurrentTime,
		PrepStartTime: s.CurrentTime.Add(time.Minute * time.Duration(s.Rng.Intn(5))),
		Status:        "placed",
	}

	order.PickupTime = order.PrepStartTime.Add(time.Minute * time.Duration(prepTime))
	return order
}

func (s *Simulator) createAndAddOrder(user *models.User) (*models.Order, error) {
	// select a restaurant
	restaurant := s.selectRestaurant(user)
	if restaurant == nil {
		// no suitable restaurant found, maybe schedule a retry later
		s.EventQueue.Enqueue(&models.Event{
			Time: s.CurrentTime.Add(15 * time.Minute),
			Type: models.EventPlaceOrder,
			Data: user,
		})
		return nil, fmt.Errorf("no suitable restaurant found")
	}

	// create a new order
	order := s.createOrder(user)
	order.RestaurantID = restaurant.ID

	// add the order to OrdersByUser
	s.OrdersByUser[user.ID] = append(s.OrdersByUser[user.ID], *order)

	// Add the order to the restaurant's current orders
	restaurant.CurrentOrders = append(restaurant.CurrentOrders, *order)

	// Add the order to the simulator's orders
	s.addOrder(*order)

	// Schedule prepare order event
	s.EventQueue.Enqueue(&models.Event{
		Time: order.PrepStartTime,
		Type: models.EventPrepareOrder,
		Data: order,
	})

	return order, nil
}

func (s *Simulator) updateOrderStatuses() {
	for i, order := range s.Orders {
		switch order.Status {
		case models.OrderStatusPlaced:
			if s.CurrentTime.After(order.PrepStartTime) {
				s.Orders[i].Status = models.OrderStatusPreparing
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventPrepareOrder,
					Data: &s.Orders[i],
				})
			}
		case models.OrderStatusPreparing:
			if s.CurrentTime.After(order.PickupTime) || s.CurrentTime.Equal(order.PickupTime) {
				s.Orders[i].Status = models.OrderStatusReady
				log.Printf("Order %s is ready for pickup at %s", order.ID, s.CurrentTime.Format(time.RFC3339))
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventOrderReady,
					Data: &s.Orders[i],
				})
			}
		case models.OrderStatusReady:
			if order.DeliveryPartnerID == "" {
				// if no partner assigned, try to assign one
				s.assignDeliveryPartner(&s.Orders[i])
			} else if s.isDeliveryPartnerAtRestaurant(s.Orders[i]) {
				s.Orders[i].Status = models.OrderStatusPickedUp
				log.Printf("Order %s picked up by partner %s at %s", order.ID, order.DeliveryPartnerID, s.CurrentTime.Format(time.RFC3339))
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventPickUpOrder,
					Data: &s.Orders[i],
				})
			}
		case models.OrderStatusPickedUp:
			s.Orders[i].Status = models.OrderStatusInTransit
			s.Orders[i].InTransitTime = s.CurrentTime
			log.Printf("Order %s is now in transit at %s", order.ID, s.CurrentTime.Format(time.RFC3339))
			s.EventQueue.Enqueue(&models.Event{
				Time: s.CurrentTime,
				Type: models.EventOrderInTransit,
				Data: &s.Orders[i],
			})

		case models.OrderStatusInTransit:
			partner := s.getDeliveryPartner(order.DeliveryPartnerID)
			if partner == nil {
				log.Printf("Error: Delivery partner not found for order %s", order.ID)
				continue
			}

			user := s.getUser(order.CustomerID)
			if user == nil {
				log.Printf("Error: User not found for order %s", order.ID)
				continue
			}

			if s.isAtLocation(partner.CurrentLocation, user.Location) {
				// order has been delivered
				s.Orders[i].Status = models.OrderStatusDelivered
				s.Orders[i].ActualDeliveryTime = s.CurrentTime
				partner.Status = models.PartnerStatusAvailable
				partner.CurrentOrderID = ""
				log.Printf("Order %s delivered at %s", order.ID, s.CurrentTime.Format(time.RFC3339))
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventDeliverOrder,
					Data: &s.Orders[i],
				})
				// schedule review creation for later
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime.Add(30 * time.Minute), // assume user leaves review after 30 minutes
					Type: models.EventGenerateReview,
					Data: &s.Orders[i],
				})
			} else {
				// order is still in transit
				nextCheckTime := s.CurrentTime.Add(5 * time.Minute)
				if s.CurrentTime.After(order.EstimatedDeliveryTime) {
					log.Printf("Order %s is past its estimated delivery time. Current: %s, Estimated: %s, Next check: %s",
						order.ID, s.CurrentTime.Format(time.RFC3339), order.EstimatedDeliveryTime.Format(time.RFC3339), nextCheckTime.Format(time.RFC3339))
				} else {
					log.Printf("Order %s still in transit. Current: %s, Estimated: %s, Next check: %s",
						order.ID, s.CurrentTime.Format(time.RFC3339), order.EstimatedDeliveryTime.Format(time.RFC3339), nextCheckTime.Format(time.RFC3339))
				}

				// Schedule next check event
				s.EventQueue.Enqueue(&models.Event{
					Time: nextCheckTime,
					Type: models.EventCheckDeliveryStatus,
					Data: &s.Orders[i],
				})
			}

		case models.OrderStatusDelivered:
			// check if it's time to generate a review
			if s.CurrentTime.Sub(s.Orders[i].ActualDeliveryTime) >= s.Config.ReviewGenerationDelay {
				if s.shouldGenerateReview(&s.Orders[i]) {
					s.handleGenerateReview(&s.Orders[i])
				}
			}
		}
	}
}

func (s *Simulator) shouldPlaceOrder(user *models.User) bool {
	hourFactor := 1.0
	if s.isPeakHour(s.CurrentTime) {
		hourFactor = s.Config.PeakHourFactor
	}
	if s.isWeekend(s.CurrentTime) {
		hourFactor *= s.Config.WeekendFactor
	}

	orderProbability := user.OrderFrequency * hourFactor / (24 * 60) // Convert to per-minute probability
	return s.Rng.Float64() < orderProbability
}

func (s *Simulator) generateNextOrderTime(user *models.User) time.Time {
	// Base time interval (in hours) derived from user's order frequency
	baseInterval := 24.0 / user.OrderFrequency

	// Adjust interval based on time of day
	hourOfDay := float64(s.CurrentTime.Hour())
	var timeOfDayFactor float64
	switch {
	case hourOfDay >= 7 && hourOfDay < 10: // Breakfast
		timeOfDayFactor = 0.8
	case hourOfDay >= 12 && hourOfDay < 14: // Lunch
		timeOfDayFactor = 0.6
	case hourOfDay >= 18 && hourOfDay < 21: // Dinner
		timeOfDayFactor = 0.5
	case hourOfDay >= 22 || hourOfDay < 6: // Late night
		timeOfDayFactor = 1.5
	default:
		timeOfDayFactor = 1.0
	}

	// Adjust interval based on day of week
	dayOfWeek := s.CurrentTime.Weekday()
	var dayOfWeekFactor float64
	if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
		dayOfWeekFactor = 0.9 // More likely to order on weekends
	} else {
		dayOfWeekFactor = 1.1
	}

	// Apply factors to base interval
	adjustedInterval := baseInterval * timeOfDayFactor * dayOfWeekFactor

	// Add some randomness (±20% of the adjusted interval)
	randomFactor := 0.8 + (0.4 * s.Rng.Float64())
	finalInterval := adjustedInterval * randomFactor

	// Convert interval to duration
	duration := time.Duration(finalInterval * float64(time.Hour))

	// Calculate next order time
	nextOrderTime := s.CurrentTime.Add(duration)

	// Ensure the next order time is not before the current time
	if nextOrderTime.Before(s.CurrentTime) {
		nextOrderTime = s.CurrentTime.Add(15 * time.Minute)
	}

	return nextOrderTime
}

func (s *Simulator) getPartnerCurrentOrder(partner *models.DeliveryPartner) *models.Order {
	if partner.CurrentOrderID == "" {
		return nil
	}
	for i := len(s.Orders) - 1; i >= 0; i-- {
		order := &s.Orders[i]
		if order.DeliveryPartnerID == partner.ID &&
			(order.Status == "picked_up" || order.Status == "ready_for_pickup") {
			return order
		}
	}
	log.Printf("Warning: Order %s not found for partner %s", partner.CurrentOrderID, partner.ID)
	return nil
}

func (s *Simulator) cancelStaleOrders() {
	maxOrderDuration := 3 * time.Hour
	for i, order := range s.Orders {
		if order.Status != models.OrderStatusDelivered && order.Status != models.OrderStatusCancelled {
			if s.CurrentTime.Sub(order.OrderPlacedAt) > maxOrderDuration {
				s.Orders[i].Status = models.OrderStatusCancelled
				log.Printf("Order %s cancelled due to timeout. Placed at: %s, Current time: %s",
					order.ID, order.OrderPlacedAt.Format(time.RFC3339), s.CurrentTime.Format(time.RFC3339))

				// free up the assigned delivery partner
				if order.DeliveryPartnerID != "" {
					for j, partner := range s.DeliveryPartners {
						if partner.ID == order.DeliveryPartnerID {
							s.DeliveryPartners[j].Status = models.PartnerStatusAvailable
							s.DeliveryPartners[j].CurrentOrderID = ""
							break
						}
					}
				}
			}
		}
	}
}

func (s *Simulator) removeCompletedOrders() {
	var activeOrders []models.Order
	for _, order := range s.Orders {
		if order.Status != models.OrderStatusDelivered && order.Status != models.OrderStatusCancelled {
			activeOrders = append(activeOrders, order)
		}
	}
	s.Orders = activeOrders
}

func (s *Simulator) assignDeliveryPartner(order *models.Order) {
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Error: Restaurant not found for order %s", order.ID)
		return
	}
	availablePartners := s.getAvailablePartnersNear(restaurant.Location)
	log.Printf("Attempting to assign partner for order %s. Available partners: %d", order.ID, len(availablePartners))
	if len(availablePartners) > 0 {
		selectedPartner := availablePartners[s.Rng.Intn(len(availablePartners))]
		if selectedPartner != nil {
			order.DeliveryPartnerID = selectedPartner.ID
			// update the partner in the slice
			for i, p := range s.DeliveryPartners {
				if p.ID == selectedPartner.ID {
					s.DeliveryPartners[i].Status = models.PartnerStatusEnRoutePickup
					s.DeliveryPartners[i].CurrentOrderID = order.ID
					log.Printf("Assigned partner %s to order %s", selectedPartner.ID, order.ID)
					break
				}
			}

			// update the order in the Orders slice
			for i, o := range s.Orders {
				if o.ID == order.ID {
					s.Orders[i] = *order
					break
				}
			}

			// set the estimated delivery time
			order.EstimatedDeliveryTime = s.estimateDeliveryTime(selectedPartner, order)

			s.notifyDeliveryPartner(selectedPartner, order)
			log.Printf("Assigned partner %s to order %s. Estimated delivery time: %s",
				selectedPartner.ID, order.ID, order.EstimatedDeliveryTime.Format(time.RFC3339))
		}
	} else {
		// if no partners are available, schedule a retry
		retryTime := s.CurrentTime.Add(5 * time.Minute)
		s.EventQueue.Enqueue(&models.Event{
			Time: retryTime,
			Type: models.EventAssignDeliveryPartner,
			Data: order,
		})
		log.Printf("No available delivery partners for order %s, scheduling retry at %s",
			order.ID, retryTime.Format(time.RFC3339))
	}
}

func (s *Simulator) getDeliveryPartner(partnerID string) *models.DeliveryPartner {
	for i, partner := range s.DeliveryPartners {
		if partner.ID == partnerID {
			return s.DeliveryPartners[i]
		}
	}
	return nil
}

func (s *Simulator) getAvailablePartnersNear(location models.Location) []*models.DeliveryPartner {
	availablePartners := make([]*models.DeliveryPartner, 0)
	for i := range s.DeliveryPartners {
		partner := s.DeliveryPartners[i]
		isNear := s.isNearLocation(partner.CurrentLocation, location)
		log.Printf("Partner %s status: %s, isNear: %v", partner.ID, partner.Status, isNear)
		if partner.Status == models.PartnerStatusAvailable && isNear {
			availablePartners = append(availablePartners, partner)
		}
	}
	log.Printf("Found %d available partners near location %v", len(availablePartners), location)
	return availablePartners
}

func (s *Simulator) isDeliveryPartnerAtRestaurant(order models.Order) bool {
	partner := s.getDeliveryPartner(order.DeliveryPartnerID)
	restaurant := s.getRestaurant(order.RestaurantID)
	if partner == nil || restaurant == nil {
		return false
	}
	return s.isAtLocation(partner.CurrentLocation, restaurant.Location)
}

func (s *Simulator) notifyDeliveryPartner(partner *models.DeliveryPartner, order *models.Order) {
	// In a real system, this would send a notification to the delivery partner
	// For our simulation, we'll update the partner's status and schedule their movement

	partner.Status = models.PartnerStatusEnRoutePickup

	// Calculate estimated arrival time at the restaurant
	restaurant := s.getRestaurant(order.RestaurantID)
	arrivalTime := s.estimateArrivalTime(partner.CurrentLocation, restaurant.Location)

	// Schedule the partner's arrival at the restaurant
	s.EventQueue.Enqueue(&models.Event{
		Time: arrivalTime,
		Type: models.EventPickUpOrder,
		Data: order,
	})

	log.Printf("Delivery partner %s notified about order %s, estimated arrival time: %s",
		partner.ID, order.ID, arrivalTime.Format(time.RFC3339))
}

func (s *Simulator) updateDeliveryPartnerLocations() {
	for i, partner := range s.DeliveryPartners {
		var newLocation models.Location
		var locationUpdated bool
		var speed float64
		switch partner.Status {
		case models.PartnerStatusAvailable:
			newLocation = s.moveTowardsHotspot(partner)
			locationUpdated = true
		case models.PartnerStatusEnRoutePickup, models.PartnerStatusEnRouteDelivery:
			order := s.getPartnerCurrentOrder(partner)
			if order == nil {
				log.Printf("Warning: Partner %s has status %s but no current order", partner.ID, partner.Status)
				continue
			}

			var destination models.Location
			if partner.Status == models.PartnerStatusEnRoutePickup {
				restaurant := s.getRestaurant(order.RestaurantID)
				if restaurant == nil {
					log.Printf("Error: Restaurant not found for order %s", order.ID)
					continue
				}
				destination = restaurant.Location
			} else {
				user := s.getUser(order.CustomerID)
				if user == nil {
					log.Printf("Error: User not found for order %s", order.ID)
					continue
				}
				destination = user.Location
			}

			newLocation = s.moveTowards(partner.CurrentLocation, destination)
			locationUpdated = true

			if s.isAtLocation(newLocation, destination) {
				if partner.Status == models.PartnerStatusEnRoutePickup {
					s.DeliveryPartners[i].Status = models.PartnerStatusWaitingForPickup
					log.Printf("Partner %s arrived at restaurant for order %s", partner.ID, order.ID)
				} else {
					s.DeliveryPartners[i].Status = models.PartnerStatusAvailable
					s.DeliveryPartners[i].CurrentOrderID = ""
					log.Printf("Partner %s completed delivery of order %s", partner.ID, order.ID)
					s.handleDeliverOrder(order)
				}
			}
		}

		if locationUpdated {
			timeDiff := s.CurrentTime.Sub(partner.LastUpdateTime).Hours()
			if timeDiff > 0 {
				distance := s.calculateDistance(partner.CurrentLocation, newLocation)
				speed = distance / timeDiff
			}

			s.DeliveryPartners[i].CurrentLocation = newLocation
			s.DeliveryPartners[i].Speed = speed
			s.DeliveryPartners[i].LastUpdateTime = s.CurrentTime

			s.EventQueue.Enqueue(&models.Event{
				Time: s.CurrentTime,
				Type: models.EventUpdatePartnerLocation,
				Data: &models.PartnerLocationUpdate{
					PartnerID:   partner.ID,
					NewLocation: newLocation,
					Speed:       speed,
					OrderID:     s.DeliveryPartners[i].CurrentOrderID,
				},
			})
		}
	}
}

func (s *Simulator) estimateArrivalTime(from, to models.Location) time.Time {
	distance := s.calculateDistance(from, to)
	travelTime := distance / s.Config.PartnerMoveSpeed // Assuming PartnerMoveSpeed is in km/hour

	// Add some variability to the travel time
	variability := 0.2 // 20% variability
	actualTravelTime := travelTime * (1 + (s.Rng.Float64()*2-1)*variability)

	return s.CurrentTime.Add(time.Duration(actualTravelTime * float64(time.Hour)))
}

func (s *Simulator) estimateDeliveryTime(partner *models.DeliveryPartner, order *models.Order) time.Time {
	user := s.getUser(order.CustomerID)
	if user == nil {
		log.Printf("Warning: User not found for order %s. Using default delivery estimate.", order.ID)
		return s.CurrentTime.Add(30 * time.Minute)
	}

	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant == nil {
		log.Printf("Warning: Restaurant not found for order %s. Using default delivery estimate.", order.ID)
		return s.CurrentTime.Add(30 * time.Minute)
	}

	// estimate time from current location to restaurant (if not already there)
	timeToRestaurant := time.Duration(0)
	if !s.isAtLocation(partner.CurrentLocation, restaurant.Location) {
		timeToRestaurant = s.estimateArrivalTime(partner.CurrentLocation, restaurant.Location).Sub(s.CurrentTime)
	}

	// estimate time from restaurant to user
	timeToUser := s.estimateArrivalTime(restaurant.Location, user.Location).Sub(s.CurrentTime)

	// add some buffer time for order handoff at restaurant and to customer, for finding parking space etc
	bufferTime := 5 * time.Minute

	// calculate total estimated time
	totalEstimatedTime := timeToRestaurant + timeToUser + bufferTime

	// add some overall variability to account for unforeseen circumstances
	variability := 0.1 // 10% variability
	adjustedTime := time.Duration(float64(totalEstimatedTime) * (1 + (s.Rng.Float64()*2-1)*variability))

	estimatedTime := s.CurrentTime.Add(adjustedTime)

	if estimatedTime.IsZero() || estimatedTime.Before(s.CurrentTime) {
		log.Printf("Warning: Invalid estimated delivery time for order %s. Setting to current time + 30 minutes.", order.ID)
		estimatedTime = s.CurrentTime.Add(30 * time.Minute)
	}

	return estimatedTime
}

func (s *Simulator) scheduleRouteUpdates(order *models.Order, partner *models.DeliveryPartner, user *models.User, estimatedArrivalTime time.Time) {
	// calculate the number of updates (e.g., every 5 minutes)
	duration := estimatedArrivalTime.Sub(s.CurrentTime)
	updateInterval := 5 * time.Minute
	numUpdates := int(duration / updateInterval)

	for i := 1; i <= numUpdates; i++ {
		updateTime := s.CurrentTime.Add(time.Duration(i) * updateInterval)
		s.EventQueue.Enqueue(&models.Event{
			Time: updateTime,
			Type: models.EventUpdatePartnerLocation,
			Data: &models.PartnerLocationUpdate{
				PartnerID:   partner.ID,
				OrderID:     order.ID,
				NewLocation: s.interpolateLocation(partner.CurrentLocation, user.Location, float64(i)/float64(numUpdates+1)),
			},
		})
	}
}

func (s *Simulator) interpolateLocation(start, end models.Location, fraction float64) models.Location {
	return models.Location{
		Lat: start.Lat + (end.Lat-start.Lat)*fraction,
		Lon: start.Lon + (end.Lon-start.Lon)*fraction,
	}
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

func (s *Simulator) selectMenuItems(restaurant *models.Restaurant, user *models.User) []string {
	// Define meal types
	// mealTypes := []string{"appetizer", "main course", "side dish", "dessert", "drink"}

	// Decide on the meal composition
	var mealComposition []string
	if s.Rng.Float32() < 0.7 { // 70% chance of a full meal
		mealComposition = []string{"main course", "side dish", "drink"}
		if s.Rng.Float32() < 0.3 { // 30% chance to add an appetizer
			mealComposition = append(mealComposition, "appetizer")
		}
		if s.Rng.Float32() < 0.2 { // 20% chance to add a dessert
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
			randValue := s.Rng.Float64() * totalProb
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

	// Calculate discount
	var discountAmount float64
	if discountableTotal >= s.Config.MinOrderForDiscount {
		discountAmount = discountableTotal * s.Config.DiscountPercentage
		if discountAmount > s.Config.MaxDiscountAmount {
			discountAmount = s.Config.MaxDiscountAmount
		}
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

func (s *Simulator) updateRestaurantMetrics(restaurant *models.Restaurant) {
	// Update average prep time
	totalPrepTime := 0.0
	for _, order := range restaurant.CurrentOrders {
		if order.PrepStartTime.After(time.Time{}) && order.PickupTime.After(time.Time{}) {
			totalPrepTime += order.PickupTime.Sub(order.PrepStartTime).Minutes()
		}
	}
	if len(restaurant.CurrentOrders) > 0 {
		restaurant.AvgPrepTime = totalPrepTime / float64(len(restaurant.CurrentOrders))
	}

	// Update restaurant efficiency
	restaurant.PickupEfficiency = s.adjustPickupEfficiency(restaurant)

	// Update restaurant capacity
	restaurant.Capacity = int(float64(restaurant.Capacity) * restaurant.PickupEfficiency)
}

func (s *Simulator) adjustRestaurantCapacity(restaurant *models.Restaurant) int {
	// Base capacity
	baseCapacity := restaurant.Capacity

	// Time-based adjustment
	timeAdjustment := s.getTimeBasedAdjustment(s.CurrentTime)

	// Demand-based adjustment
	demandAdjustment := s.getDemandBasedAdjustment(restaurant)

	// Day of week adjustment
	dayAdjustment := s.getDayOfWeekAdjustment(s.CurrentTime)

	// Calculate new capacity
	newCapacity := int(float64(baseCapacity) * timeAdjustment * demandAdjustment * dayAdjustment)

	// Ensure capacity doesn't go below a minimum threshold or above a maximum
	minCapacity := max(1, int(float64(baseCapacity)*0.5)) // At least 1, or 50% of base capacity
	maxCapacity := int(float64(baseCapacity) * 1.5)       // 150% of base capacity

	newCapacity = max(minCapacity, min(newCapacity, maxCapacity))

	return newCapacity
}

func (s *Simulator) getTimeBasedAdjustment(currentTime time.Time) float64 {
	hour := currentTime.Hour()
	switch {
	case hour >= 11 && hour < 14: // Lunch rush
		return 1.3
	case hour >= 18 && hour < 21: // Dinner rush
		return 1.4
	case hour >= 23 || hour < 6: // Late night / Early morning
		return 0.7
	default:
		return 1.0
	}
}

func (s *Simulator) getDemandBasedAdjustment(restaurant *models.Restaurant) float64 {
	// Calculate recent order volume (last hour)
	recentOrders := 0
	for _, order := range restaurant.CurrentOrders {
		if s.CurrentTime.Sub(order.OrderPlacedAt) <= 1*time.Hour {
			recentOrders++
		}
	}

	// Calculate the ratio of recent orders to current capacity
	demandRatio := float64(recentOrders) / float64(restaurant.Capacity)

	switch {
	case demandRatio > 0.9: // Very high demand
		return 1.2
	case demandRatio > 0.7: // High demand
		return 1.1
	case demandRatio < 0.3: // Low demand
		return 0.9
	default:
		return 1.0
	}
}

func (s *Simulator) getDayOfWeekAdjustment(currentTime time.Time) float64 {
	switch currentTime.Weekday() {
	case time.Friday, time.Saturday:
		return 1.2 // Increase capacity on weekends
	case time.Sunday:
		return 1.1 // Slight increase on Sundays
	default:
		return 1.0
	}
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
	randomFactor := 1 + (s.Rng.Float64()-0.5)*0.1 // ±5% random variation

	finalPrepTime := adjustedTime * loadFactor * randomFactor

	return math.Max(finalPrepTime, restaurant.MinPrepTime)
}

func (s *Simulator) isNearLocation(loc1, loc2 models.Location) bool {
	distance := s.calculateDistance(loc1, loc2)

	// base threshold
	threshold := s.Config.NearLocationThreshold

	// adjust threshold based on time of day (e.g., wider range during off-peak hours)
	if !s.isPeakHour(s.CurrentTime) {
		threshold *= 1.5
	}

	// adjust threshold based on urban density
	if s.isUrbanArea(loc1) && s.isUrbanArea(loc2) {
		threshold *= 0.8 // smaller threshold in urban areas
	}

	return distance <= threshold*2
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

func (s *Simulator) moveTowardsHotspot(partner *models.DeliveryPartner) models.Location {
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
	nearestHotspot.Location.Lat += (s.Rng.Float64() - 0.5) * jitter
	nearestHotspot.Location.Lon += (s.Rng.Float64() - 0.5) * jitter

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
	distance := s.calculateDistance(loc1, loc2)
	return distance < 0.1 // consider locations the same if they're within 100 meters
}

func (s *Simulator) adjustOrderFrequency(user *models.User) float64 {
	recentOrders := s.getRecentOrders(user.ID, s.Config.UserBehaviourWindow)
	if len(recentOrders) <= 1 {
		return user.OrderFrequency // No change if there's 0 or 1 recent order
	}

	// calculate time between orders
	var totalTimeBetween float64
	for i := 1; i < len(recentOrders); i++ {
		timeBetween := recentOrders[i].OrderPlacedAt.Sub(recentOrders[i-1].OrderPlacedAt).Hours()
		totalTimeBetween += timeBetween
	}

	// avoid division by zero
	if totalTimeBetween == 0 {
		return user.OrderFrequency // No change if all orders were placed at the same time
	}

	avgTimeBetween := totalTimeBetween / float64(len(recentOrders)-1)

	// avoid division by zero
	if avgTimeBetween == 0 {
		return user.OrderFrequency // No change if average time between orders is zero
	}

	// convert to frequency (orders per day)
	newFrequency := 24 / avgTimeBetween

	// check for unreasonably high frequencies
	if newFrequency > 24 { // More than 24 orders per day seems unrealistic
		newFrequency = 24
	}

	// gradually adjust towards new frequency
	adjustmentRate := 0.2 // 20% adjustment towards new frequency
	adjustedFrequency := user.OrderFrequency + (newFrequency-user.OrderFrequency)*adjustmentRate

	// ensure the frequency is within a reasonable range
	minFrequency := 0.01 // at least one order every 100 days
	maxFrequency := 5.0  // at most 5 orders per day
	return math.Max(minFrequency, math.Min(maxFrequency, adjustedFrequency))
}

func (s *Simulator) adjustPrepTime(restaurant *models.Restaurant) float64 {
	currentLoad := float64(len(restaurant.CurrentOrders)) / float64(restaurant.Capacity)
	loadFactor := 1 + (currentLoad * s.Config.RestaurantLoadFactor)

	// Adjust prep time based on current load
	adjustedPrepTime := restaurant.AvgPrepTime * loadFactor

	// Ensure prep time doesn't go below minimum or become NaN
	if math.IsNaN(adjustedPrepTime) || adjustedPrepTime < restaurant.MinPrepTime {
		return restaurant.MinPrepTime
	}

	return adjustedPrepTime
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

func (s *Simulator) serializeInitialDataToCSV(outputFolder string) error {
	// clean the output folder before serializing
	if err := cleanOutputFolder(outputFolder); err != nil {
		return fmt.Errorf("failed to clean output folder: %w", err)
	}

	// serialise restaurants
	if err := s.serializeRestaurantsToCSV(filepath.Join(outputFolder, "restaurants.csv")); err != nil {
		return fmt.Errorf("failed to serialize restaurants: %w", err)
	}

	// serialise users
	if err := s.serializeUsersToCSV(filepath.Join(outputFolder, "users.csv")); err != nil {
		return fmt.Errorf("failed to serialize users: %w", err)
	}

	// serialise delivery partners
	if err := s.serializeDeliveryPartnersToCSV(filepath.Join(outputFolder, "delivery_partners.csv")); err != nil {
		return fmt.Errorf("failed to serialize delivery partners: %w", err)
	}

	// serialise menu items
	if err := s.serializeMenuItemsToCSV(filepath.Join(outputFolder, "menu_items.csv")); err != nil {
		return fmt.Errorf("failed to serialize menu items: %w", err)
	}

	return nil
}

func (s *Simulator) serializeRestaurantsToCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"ID", "Name", "Latitude", "Longitude", "Cuisines", "MenuItemIds", "Rating", "TotalRatings",
		"PrepTime", "MinPrepTime", "AvgPrepTime", "PickupEfficiency", "Capacity",
		"Host", "Phone", "Town", "SlugName", "WebsiteLogoURL", "Offline", "Currency",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data for each restaurant
	for _, restaurant := range s.Restaurants {
		row := []string{
			restaurant.ID,
			restaurant.Name,
			strconv.FormatFloat(restaurant.Location.Lat, 'f', 6, 64),
			strconv.FormatFloat(restaurant.Location.Lon, 'f', 6, 64),
			strings.Join(restaurant.Cuisines, "|"),
			strings.Join(restaurant.MenuItems, "|"),
			strconv.FormatFloat(restaurant.Rating, 'f', 2, 64),
			strconv.FormatFloat(restaurant.TotalRatings, 'f', 0, 64),
			strconv.FormatFloat(restaurant.PrepTime, 'f', 2, 64),
			strconv.FormatFloat(restaurant.MinPrepTime, 'f', 2, 64),
			strconv.FormatFloat(restaurant.AvgPrepTime, 'f', 2, 64),
			strconv.FormatFloat(restaurant.PickupEfficiency, 'f', 2, 64),
			strconv.Itoa(restaurant.Capacity),
			restaurant.Host,
			restaurant.Phone,
			restaurant.Town,
			restaurant.SlugName,
			restaurant.WebsiteLogoURL,
			restaurant.Offline,
			strconv.Itoa(restaurant.Currency),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (s *Simulator) serializeUsersToCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"ID", "Name", "Latitude", "Longitude", "Preferences", "DietaryRestrictions", "OrderFrequency"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, user := range s.Users {
		row := []string{
			user.ID,
			user.Name,
			strconv.FormatFloat(user.Location.Lat, 'f', 6, 64),
			strconv.FormatFloat(user.Location.Lon, 'f', 6, 64),
			strings.Join(user.Preferences, "|"),
			strings.Join(user.DietaryRestrictions, "|"),
			strconv.FormatFloat(user.OrderFrequency, 'f', 4, 64),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (s *Simulator) serializeDeliveryPartnersToCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"ID", "Name", "Latitude", "Longitude", "Status", "Rating", "TotalRatings"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, partner := range s.DeliveryPartners {
		row := []string{
			partner.ID,
			partner.Name,
			strconv.FormatFloat(partner.CurrentLocation.Lat, 'f', 6, 64),
			strconv.FormatFloat(partner.CurrentLocation.Lon, 'f', 6, 64),
			partner.Status,
			strconv.FormatFloat(partner.Rating, 'f', 2, 64),
			strconv.FormatFloat(partner.TotalRatings, 'f', 0, 64),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func (s *Simulator) serializeMenuItemsToCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"ID", "RestaurantID", "Name", "Description", "Price", "Type", "Popularity", "PrepComplexity", "Ingredients"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, menuItem := range s.MenuItems {
		row := []string{
			menuItem.ID,
			menuItem.RestaurantID,
			menuItem.Name,
			menuItem.Description,
			strconv.FormatFloat(menuItem.Price, 'f', 2, 64),
			menuItem.Type,
			strconv.FormatFloat(menuItem.Popularity, 'f', 2, 64),
			strconv.FormatFloat(menuItem.PrepComplexity, 'f', 2, 64),
			strings.Join(menuItem.Ingredients, "|"),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
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

func updateRating(currentRating, newRating, alpha float64) float64 {
	updatedRating := (alpha * newRating) + ((1 - alpha) * currentRating)
	return math.Max(1, math.Min(5, updatedRating))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func safeUnixTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func cleanOutputFolder(outputFolder string) error {
	err := os.RemoveAll(outputFolder)
	if err != nil {
		return fmt.Errorf("failed to remove output folder: %w", err)
	}

	err = os.MkdirAll(outputFolder, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create output folder: %w", err)
	}

	return nil
}
