package simulator

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/factories"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"hash/fnv"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

const earthRadiusKm = 6371.0 // Earth's radius in kilometers
const deliveryThreshold = 0.1
const batchSize = 100

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
	return s.safeFloat64() < baseProbability
}

func (s *Simulator) createReview(order *models.Order) models.Review {
	user := s.getUser(order.CustomerID)
	pattern := ReviewPatterns[user.Segment]

	// calculate base ratings
	foodRating := s.calculateFoodRating(order, pattern.RatingBias)
	deliveryRating := s.calculateDeliveryRating(order)

	// apply contextual adjustments
	weather := s.getCurrentWeather()
	if impact, exists := WeatherImpact[weather]; exists {
		deliveryRating = math.Max(1, deliveryRating+impact.RatingAdjustment)
	}

	// calculate overall rating
	overallRating := s.calculateOverallRating(foodRating, deliveryRating, order)

	// generate appropriate comment
	comment := s.generateReviewComment(order, pattern, foodRating, deliveryRating)

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

func (s *Simulator) calculateGrowthRate(baseRate float64, currentTime time.Time) float64 {
	dayOfYear := float64(currentTime.YearDay())
	seasonalFactor := math.Sin(2 * math.Pi * dayOfYear / 365.0)

	// higher growth during summer months (assuming Northern hemisphere)
	if currentTime.Month() >= time.June && currentTime.Month() <= time.August {
		seasonalFactor *= 1.5
	}

	// weekend boost
	if currentTime.Weekday() == time.Friday || currentTime.Weekday() == time.Saturday {
		seasonalFactor *= 1.2
	}

	return baseRate + (seasonalFactor * 0.05) // 5% seasonal variation
}

func (s *Simulator) growUsers() {
	// check if user growth rate is specified
	if s.Config.UserGrowthRate == 0 {
		return // No growth if rate is not specified
	}

	// calculate daily growth rate
	dailyGrowthRate := math.Pow(1+s.Config.UserGrowthRate, 1.0/365.0) - 1

	// calculate the number of days since the start of the simulation
	daysSinceStart := s.CurrentTime.Sub(s.Config.StartDate).Hours() / 24

	// calculate the expected number of users at this point in the simulation
	expectedUsers := float64(s.Config.InitialUsers) * math.Pow(1+dailyGrowthRate, daysSinceStart)

	// calculate how many new users to add
	newUsersToAdd := int(expectedUsers) - len(s.Users)

	if newUsersToAdd > 0 {
		userFactory := &factories.UserFactory{}
		for i := 0; i < newUsersToAdd; i++ {
			newUser := userFactory.CreateUser(s.Config)
			s.Users = append(s.Users, newUser)

			// schedule the first order for this new user
			nextOrderTime := s.generateNextOrderTime(newUser)
			s.EventQueue.Enqueue(&models.Event{
				Time: nextOrderTime,
				Type: models.EventPlaceOrder,
				Data: newUser,
			})
		}
		log.Printf("Added %d new users. Total users: %d", newUsersToAdd, len(s.Users))
	}
}

func (s *Simulator) updatePopulationWithGrowth(growthRate float64) {
	// calculate expected new users
	currentUsers := float64(len(s.Users))
	expectedGrowth := currentUsers * growthRate

	// add new users (rounded to nearest integer)
	newUsersCount := int(math.Round(expectedGrowth))
	if newUsersCount > 0 {
		userFactory := &factories.UserFactory{}
		for i := 0; i < newUsersCount; i++ {
			newUser := userFactory.CreateUser(s.Config)
			s.Users = append(s.Users, newUser)

			// Schedule first order for new user
			nextOrderTime := s.generateNextOrderTime(newUser)
			s.EventQueue.Enqueue(&models.Event{
				Time: nextOrderTime,
				Type: models.EventPlaceOrder,
				Data: newUser,
			})
		}
		log.Printf("Added %d new users based on growth rate of %.2f%%", newUsersCount, growthRate*100)
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

func (s *Simulator) calculateFoodRating(order *models.Order, bias RatingBias) float64 {
	baseRating := bias.FoodBase

	// price satisfaction impact
	priceImpact := s.calculatePriceSatisfaction(order) * bias.PriceInfluence

	// restaurant historical performance
	restaurant := s.getRestaurant(order.RestaurantID)
	historicalImpact := 0.0
	if restaurant != nil {
		historicalDiff := restaurant.Rating - 4.0
		historicalImpact = historicalDiff * 0.2
	}

	// order complexity impact
	complexityImpact := s.calculateOrderComplexityImpact(order)

	finalRating := baseRating + priceImpact + historicalImpact + complexityImpact

	// add some randomness (±0.5)
	randomFactor := s.safeFloat64() - 0.5

	return math.Max(1, math.Min(5, finalRating+randomFactor))
}

func (s *Simulator) calculatePriceSatisfaction(order *models.Order) float64 {
	user := s.getUser(order.CustomerID)
	if user == nil {
		return 0
	}

	profile := user.BehaviorProfile
	if order.TotalAmount > profile.PriceThresholds.Max {
		return -0.5
	} else if order.TotalAmount < profile.PriceThresholds.Min {
		return 0.2
	}

	// calculate how close the order amount is to the user's target price
	priceDiff := math.Abs(order.TotalAmount - profile.PriceThresholds.Target)
	if priceDiff < 5 {
		return 0.3
	}

	return 0
}

func (s *Simulator) calculateRestaurantPopularity(restaurant *models.Restaurant) RestaurantPopularityMetrics {
	metrics := RestaurantPopularityMetrics{
		TimeBasedDemand:  make(map[int]float64),
		CustomerSegments: make(map[string]float64),
	}

	// base popularity from rating and order history
	metrics.BasePopularity = s.calculateBasePopularity(restaurant)

	// Calculate trend factor
	metrics.TrendFactor = s.calculatePopularityTrend(restaurant)

	// Calculate time-based demand patterns
	metrics.TimeBasedDemand = s.analyzeTimeBasedDemand(restaurant)

	// Calculate segment-specific appeal
	metrics.CustomerSegments = s.analyzeSegmentAppeal(restaurant)

	// Price-quality metrics
	metrics.PriceAppeal = s.calculatePriceAppeal(restaurant)
	metrics.QualityAppeal = s.calculateQualityAppeal(restaurant)
	metrics.ConsistencyAppeal = s.calculateConsistencyAppeal(restaurant)

	return metrics
}

func (s *Simulator) updateOrderGeneration(user *models.User, restaurants []*models.Restaurant) *models.Restaurant {
	restaurantScores := make(map[string]float64)

	for _, restaurant := range restaurants {
		popularity := s.calculateRestaurantPopularity(restaurant)
		marketPosition := s.analyzeMarketPosition(restaurant)
		demandFactors := s.getCurrentDemandFactors()

		// calculate base score
		score := s.calculateRestaurantScore(restaurant, user, popularity, marketPosition, demandFactors)

		// apply reputation-based adjustments
		reputationMultiplier := s.calculateReputationMultiplier(restaurant, user)
		score *= reputationMultiplier

		restaurantScores[restaurant.ID] = score
	}

	// Select restaurant probabilistically based on scores
	return s.selectRestaurantByScore(restaurants, restaurantScores)
}

func (s *Simulator) calculateRestaurantScore(
	restaurant *models.Restaurant,
	user *models.User,
	popularity RestaurantPopularityMetrics,
	position MarketPosition,
	demand DemandFactors,
) float64 {
	score := popularity.BasePopularity

	// user segment preferences
	if multiplier, exists := popularity.CustomerSegments[user.Segment]; exists {
		score *= multiplier
	}

	// time-based demand
	currentHour := s.CurrentTime.Hour()
	if multiplier, exists := popularity.TimeBasedDemand[currentHour]; exists {
		score *= multiplier
	}

	// price-quality consideration based on user segment
	priceQualityScore := s.calculatePriceQualityPreference(user, restaurant, position)
	score *= priceQualityScore

	// trend impact
	trendImpact := 1.0 + (popularity.TrendFactor * 0.2) // ±20% impact
	score *= trendImpact

	// current demand factors
	score *= demand.TimeOfDay * demand.DayOfWeek * demand.Weather *
		demand.Seasonality * demand.SpecialEvents

	return score
}

func (s *Simulator) calculateReputationMultiplier(restaurant *models.Restaurant, user *models.User) float64 {
	multiplier := 1.0

	// rating impact
	if restaurant.Rating >= 4.5 {
		multiplier *= 1.3
	} else if restaurant.Rating <= 3.0 {
		multiplier *= 0.7
	}

	// user's previous experience with the restaurant
	if previousRating := s.getUserPreviousRating(user.ID, restaurant.ID); previousRating > 0 {
		experienceMultiplier := math.Pow(previousRating/3.0, 2) // Squared to amplify effect
		multiplier *= experienceMultiplier
	}

	// consistency impact
	metrics := s.calculateReputationMetrics(restaurant)
	multiplier *= math.Pow(metrics.ConsistencyScore, 1.5) // Stronger effect for consistency

	return multiplier
}

func (s *Simulator) calculatePriceQualityPreference(
	user *models.User,
	restaurant *models.Restaurant,
	position MarketPosition,
) float64 {
	userProfile := user.BehaviorProfile
	avgPrice := s.calculateAverageItemPrice(restaurant)

	// calculate price sensitivity based on user segment
	priceSensitivity := map[string]float64{
		"frequent":   0.7, // Less sensitive
		"regular":    1.0,
		"occasional": 1.3, // More sensitive
	}[user.Segment]

	// price threshold comparison
	if avgPrice > userProfile.PriceThresholds.Max {
		return 0.5 * priceSensitivity
	} else if avgPrice < userProfile.PriceThresholds.Min {
		return 0.8
	}

	// quality expectations met?
	qualityScore := 1.0
	if position.QualityTier == "premium" && restaurant.Rating >= 4.3 {
		qualityScore = 1.2
	} else if position.QualityTier == "budget" && restaurant.Rating >= 4.0 {
		qualityScore = 1.3 // exceeding expectations for price point
	}

	return qualityScore / priceSensitivity
}

func (s *Simulator) getCurrentDemandFactors() DemandFactors {
	factors := DemandFactors{
		TimeOfDay:     1.0,
		DayOfWeek:     1.0,
		Weather:       1.0,
		Seasonality:   1.0,
		SpecialEvents: 1.0,
	}

	// time of day factors
	hour := s.CurrentTime.Hour()
	factors.TimeOfDay = map[int]float64{
		7:  1.2, // breakfast
		8:  1.3,
		12: 1.5, // lunch
		13: 1.5,
		18: 1.6, // dinner
		19: 1.7,
		20: 1.5,
	}[hour]

	if factors.TimeOfDay == 0 {
		factors.TimeOfDay = 1.0
	}

	// day of week factors
	switch s.CurrentTime.Weekday() {
	case time.Friday:
		factors.DayOfWeek = 1.4
	case time.Saturday:
		factors.DayOfWeek = 1.5
	case time.Sunday:
		factors.DayOfWeek = 1.3
	default:
		factors.DayOfWeek = 1.0
	}

	// weather impact
	weather := s.getCurrentWeather()
	factors.Weather = map[string]float64{
		"rain": 1.3,
		"snow": 1.4,
		"hot":  0.9,
		"cold": 1.2,
	}[weather]

	// seasonal factors
	month := s.CurrentTime.Month()
	if month >= time.November && month <= time.February {
		factors.Seasonality = 1.2 // winter boost for food delivery
	} else if month >= time.June && month <= time.August {
		factors.Seasonality = 0.9 // summer slight decline
	}

	// :TODO calculate special events
	return factors
}

func (s *Simulator) selectRestaurantByScore(
	restaurants []*models.Restaurant,
	scores map[string]float64,
) *models.Restaurant {
	if len(restaurants) == 0 {
		return nil
	}
	// calculate total score
	totalScore := 0.0
	for _, score := range scores {
		totalScore += score
	}

	// select probabilistically
	r := s.safeFloat64() * totalScore
	currentSum := 0.0

	for _, restaurant := range restaurants {
		currentSum += scores[restaurant.ID]
		if r <= currentSum {
			return restaurant
		}
	}

	// fallback to first restaurant
	if len(restaurants) > 0 {
		return restaurants[0]
	}

	return nil
}

func (s *Simulator) calculateOrderComplexityImpact(order *models.Order) float64 {
	if len(order.Items) <= 1 {
		return 0
	}

	totalComplexity := 0.0
	for _, itemID := range order.Items {
		if item := s.getMenuItem(itemID); item != nil {
			totalComplexity += item.PrepComplexity
		}
	}

	avgComplexity := totalComplexity / float64(len(order.Items))
	if avgComplexity > 0.8 {
		return -0.2 // higher complexity slightly reduces rating
	}

	return 0
}

func (s *Simulator) calculateOverallRating(foodRating, deliveryRating float64, order *models.Order) float64 {
	user := s.getUser(order.CustomerID)
	if user == nil {
		return (foodRating + deliveryRating) / 2
	}

	// different segments weight food vs delivery differently
	var foodWeight, deliveryWeight float64
	switch user.Segment {
	case "frequent":
		foodWeight = 0.6
		deliveryWeight = 0.4
	case "regular":
		foodWeight = 0.5
		deliveryWeight = 0.5
	case "occasional":
		foodWeight = 0.4
		deliveryWeight = 0.6
	default:
		foodWeight = 0.5
		deliveryWeight = 0.5
	}

	weightedRating := (foodRating * foodWeight) + (deliveryRating * deliveryWeight)

	// add slight randomness
	randomFactor := s.safeFloat64()*0.2 - 0.1 // ±0.1

	return math.Max(1, math.Min(5, weightedRating+randomFactor))
}

func (s *Simulator) generateReviewComment(order *models.Order, pattern ReviewPattern, foodRating, deliveryRating float64) string {
	var sentiment string
	if (foodRating+deliveryRating)/2 >= 4.0 {
		sentiment = "positive"
	} else if (foodRating+deliveryRating)/2 <= 2.5 {
		sentiment = "negative"
	} else {
		sentiment = "neutral"
	}

	// find matching comment pattern
	var selectedPattern CommentPattern
	for _, cp := range pattern.CommentPatterns {
		if cp.Sentiment == sentiment {
			selectedPattern = cp
			break
		}
	}

	// generate comment based on triggers
	var comments []string
	timeDiff := order.ActualDeliveryTime.Sub(order.EstimatedDeliveryTime).Minutes()

	if selectedPattern.Triggers != nil {
		if lateThreshold, exists := selectedPattern.Triggers["late_delivery"]; exists && timeDiff > lateThreshold {
			comments = append(comments, "Delivery was significantly delayed. ")
		}
		if earlyThreshold, exists := selectedPattern.Triggers["early_delivery"]; exists && timeDiff < -earlyThreshold {
			comments = append(comments, "Impressively quick delivery! ")
		}
		if priceThreshold, exists := selectedPattern.Triggers["high_price"]; exists && order.TotalAmount > priceThreshold {
			comments = append(comments, "Premium pricing but ")
		}
	}

	// add base comment from templates
	if len(selectedPattern.Templates) > 0 {
		baseComment := selectedPattern.Templates[s.safeIntn(len(selectedPattern.Templates))]
		comments = append(comments, baseComment)
	}

	// combine comments
	finalComment := ""
	for _, comment := range comments {
		finalComment += comment
	}

	return finalComment
}

func (s *Simulator) calculateDeliveryRating(order *models.Order) float64 {
	estimatedDeliveryTime := order.EstimatedDeliveryTime.Sub(order.OrderPlacedAt)
	actualDeliveryTime := order.ActualDeliveryTime.Sub(order.OrderPlacedAt)
	timeDifference := actualDeliveryTime - estimatedDeliveryTime

	// get location cluster
	deliveryPartner := s.getDeliveryPartner(order.DeliveryPartnerID)
	if deliveryPartner == nil {
		return 3.0 // default rating if partner not found
	}

	cluster := s.getLocationCluster(deliveryPartner.CurrentLocation)
	clusterWeight := RatingDistributions.ClusterWeights[cluster]

	// calculate base rating
	baseRating := s.calculateBaseDeliveryRating(timeDifference)

	// apply time-based weights
	timeWeight := s.getTimeBasedRatingWeight(order.OrderPlacedAt)

	// apply weather effects
	weatherWeight := s.getWeatherRatingWeight()

	// calculate final rating
	adjustedRating := baseRating * clusterWeight * timeWeight * weatherWeight

	// add some randomness (±0.5 stars)
	finalRating := adjustedRating + (s.safeFloat64() - 0.5)

	// ensure rating is between 1 and 5
	return math.Max(1, math.Min(5, finalRating))
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
	if s.safeFloat64() < 0.5 {
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

func (s *Simulator) getNearbyRestaurants(userLocation models.Location, radius float64) []*models.Restaurant {
	var nearbyRestaurants []*models.Restaurant
	for _, restaurant := range s.Restaurants {
		if distance := s.calculateDistance(userLocation, restaurant.Location); distance <= radius {
			nearbyRestaurants = append(nearbyRestaurants, restaurant)
		}
	}
	return nearbyRestaurants
}

func (s *Simulator) getRandomRestaurant() *models.Restaurant {
	restaurants := make([]*models.Restaurant, 0, len(s.Restaurants))
	for _, r := range s.Restaurants {
		restaurants = append(restaurants, r)
	}
	return restaurants[s.safeIntn(len(restaurants))]
}

func (s *Simulator) selectRestaurant(user *models.User) *models.Restaurant {
	segment := models.DefaultCustomerSegments[user.Segment]
	nearbyRestaurants := s.getNearbyRestaurants(user.Location, 5.0)

	type scoredRestaurant struct {
		restaurant *models.Restaurant
		score      float64
	}

	var scoredRestaurants []scoredRestaurant

	for _, restaurant := range nearbyRestaurants {
		popularity := s.calculateRestaurantPopularity(restaurant)
		marketPosition := s.analyzeMarketPosition(restaurant)
		demandFactors := s.getCurrentDemandFactors()

		// calculate base score
		score := s.calculateRestaurantScore(restaurant, user, popularity, marketPosition, demandFactors)

		// adjust score based on segment preferences
		for _, cuisine := range restaurant.Cuisines {
			if weight, exists := segment.CuisinePreferences[strings.ToLower(cuisine)]; exists {
				score *= 1 + weight
			}
		}

		// price sensitivity by segment
		avgItemPrice := s.calculateAverageItemPrice(restaurant)
		if avgItemPrice > segment.AvgSpend*1.2 {
			score *= 0.7 // reduce score for expensive restaurants for price-sensitive segments
		}

		scoredRestaurants = append(scoredRestaurants, scoredRestaurant{restaurant, score})
	}

	// sort by score and select probabilistically
	sort.Slice(scoredRestaurants, func(i, j int) bool {
		return scoredRestaurants[i].score > scoredRestaurants[j].score
	})

	// select with probability weighted by score
	totalScore := 0.0
	for _, sr := range scoredRestaurants {
		totalScore += sr.score
	}

	randomValue := s.safeFloat64() * totalScore
	currentSum := 0.0

	for _, sr := range scoredRestaurants {
		currentSum += sr.score
		if currentSum >= randomValue {
			return sr.restaurant
		}
	}

	// fallback to first restaurant if something goes wrong
	if len(scoredRestaurants) > 0 {
		return scoredRestaurants[0].restaurant
	}

	return s.getRandomRestaurant()
}

func (s *Simulator) calculateAverageItemPrice(restaurant *models.Restaurant) float64 {
	if restaurant == nil || len(restaurant.MenuItems) == 0 {
		return 0
	}

	var totalPrice float64
	var itemCount int
	pricesByType := make(map[string][]float64)

	// first pass: collect prices by item type
	for _, itemID := range restaurant.MenuItems {
		item := s.getMenuItem(itemID)
		if item == nil {
			continue
		}

		// store price in its category
		pricesByType[item.Type] = append(pricesByType[item.Type], item.Price)
		totalPrice += item.Price
		itemCount++
	}

	if itemCount == 0 {
		return 0
	}

	// calculate weighted average based on item types
	weightedTotal := 0.0
	weightedCount := 0.0

	// weights for different item types
	typeWeights := map[string]float64{
		"main course": 1.0, // full weight for main courses
		"appetizer":   0.7, // lower weight for appetizers
		"side dish":   0.5, // lower weight for sides
		"dessert":     0.6, // medium-low weight for desserts
		"drink":       0.4, // lowest weight for drinks
	}

	for itemType, prices := range pricesByType {
		if len(prices) == 0 {
			continue
		}

		// calculate average price for this type
		typeTotal := 0.0
		for _, price := range prices {
			typeTotal += price
		}
		typeAvg := typeTotal / float64(len(prices))

		// apply weight for this type
		weight := typeWeights[itemType]
		if weight == 0 {
			weight = 0.5 // default weight for unknown types
		}

		weightedTotal += typeAvg * weight
		weightedCount += weight
	}

	// if we have weighted prices, use them
	if weightedCount > 0 {
		return math.Round((weightedTotal/weightedCount)*100) / 100
	}

	// default to simple average
	return math.Round((totalPrice/float64(itemCount))*100) / 100
}

func (s *Simulator) getPriceRangeCategory(avgPrice float64) string {
	switch {
	case avgPrice >= 40:
		return "premium"
	case avgPrice >= 20:
		return "standard"
	default:
		return "budget"
	}
}

func (s *Simulator) getRepresentativePrice(restaurant *models.Restaurant) float64 {
	if restaurant == nil || len(restaurant.MenuItems) == 0 {
		return 0
	}

	// get all main course prices
	var mainCoursePrices []float64
	for _, itemID := range restaurant.MenuItems {
		item := s.getMenuItem(itemID)
		if item != nil && item.Type == "main course" {
			mainCoursePrices = append(mainCoursePrices, item.Price)
		}
	}

	if len(mainCoursePrices) == 0 {
		return s.calculateAverageItemPrice(restaurant)
	}

	// sort prices
	sort.Float64s(mainCoursePrices)

	// calculate median price for main courses
	medianIndex := len(mainCoursePrices) / 2
	if len(mainCoursePrices)%2 == 0 {
		return (mainCoursePrices[medianIndex-1] + mainCoursePrices[medianIndex]) / 2
	}
	return mainCoursePrices[medianIndex]
}

func (s *Simulator) analyzePriceDistribution(restaurant *models.Restaurant) struct {
	Min    float64
	Max    float64
	Median float64
	Mean   float64
	StdDev float64
} {
	var prices []float64
	for _, itemID := range restaurant.MenuItems {
		item := s.getMenuItem(itemID)
		if item != nil {
			prices = append(prices, item.Price)
		}
	}

	if len(prices) == 0 {
		return struct {
			Min    float64
			Max    float64
			Median float64
			Mean   float64
			StdDev float64
		}{0, 0, 0, 0, 0}
	}

	// sort prices for min, max, median
	sort.Float64s(prices)

	// calculate mean
	sum := 0.0
	for _, price := range prices {
		sum += price
	}
	mean := sum / float64(len(prices))

	// calculate standard deviation
	sumSquaredDiff := 0.0
	for _, price := range prices {
		diff := price - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(len(prices)))

	// calculate median
	medianIndex := len(prices) / 2
	var median float64
	if len(prices)%2 == 0 {
		median = (prices[medianIndex-1] + prices[medianIndex]) / 2
	} else {
		median = prices[medianIndex]
	}

	return struct {
		Min    float64
		Max    float64
		Median float64
		Mean   float64
		StdDev float64
	}{
		Min:    prices[0],
		Max:    prices[len(prices)-1],
		Median: median,
		Mean:   mean,
		StdDev: stdDev,
	}
}

func (s *Simulator) calculatePriceCompetitiveness(restaurant *models.Restaurant) float64 {
	if restaurant == nil {
		return 0
	}

	// get nearby restaurants
	nearbyRestaurants := s.getNearbyRestaurants(restaurant.Location, 5.0) // 5km radius

	if len(nearbyRestaurants) <= 1 {
		return 1.0 // no competition nearby
	}

	restaurantAvgPrice := s.calculateAverageItemPrice(restaurant)
	if restaurantAvgPrice == 0 {
		return 0
	}

	// calculate average prices for nearby restaurants with same cuisines
	var competitorPrices []float64
	for _, competitor := range nearbyRestaurants {
		if competitor.ID == restaurant.ID {
			continue
		}

		// check if cuisines overlap
		if s.hasCuisineOverlap(restaurant, competitor) {
			competitorPrice := s.calculateAverageItemPrice(competitor)
			if competitorPrice > 0 {
				competitorPrices = append(competitorPrices, competitorPrice)
			}
		}
	}

	if len(competitorPrices) == 0 {
		return 1.0 // no direct competitors
	}

	// calculate average competitor price
	var totalCompetitorPrice float64
	for _, price := range competitorPrices {
		totalCompetitorPrice += price
	}
	avgCompetitorPrice := totalCompetitorPrice / float64(len(competitorPrices))

	// calculate competitiveness score
	// 1.0  => average, <1.0 => cheaper, >1.0 => more expensive
	competitiveness := restaurantAvgPrice / avgCompetitorPrice

	return math.Round(competitiveness*100) / 100
}

func (s *Simulator) hasCuisineOverlap(r1, r2 *models.Restaurant) bool {
	cuisineMap := make(map[string]bool)
	for _, cuisine := range r1.Cuisines {
		cuisineMap[cuisine] = true
	}

	for _, cuisine := range r2.Cuisines {
		if cuisineMap[cuisine] {
			return true
		}
	}
	return false
}

func (s *Simulator) calculateDistance(loc1, loc2 models.Location) float64 {
	// convert latitude and longitude from degrees to radians
	lat1 := degreesToRadians(loc1.Lat)
	lon1 := degreesToRadians(loc1.Lon)
	lat2 := degreesToRadians(loc2.Lat)
	lon2 := degreesToRadians(loc2.Lon)

	// haversine formula
	dlat := lat2 - lat1
	dlon := lon2 - lon1
	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// calculate the distance
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
	randomFactor := 1 + (s.safeFloat64()-0.5)*s.Config.TrafficVariability
	return baseTraffic * randomFactor
}

func (s *Simulator) generateOrders() {
	for _, user := range s.Users {
		if s.shouldPlaceOrder(user) {
			nearbyRestaurants := s.getNearbyRestaurants(user.Location, 5.0)
			if len(nearbyRestaurants) == 0 {
				continue
			}
			// select restaurant using reputation-based system
			restaurant := s.updateOrderGeneration(user, nearbyRestaurants)
			if restaurant == nil {
				continue
			}

			order := s.createOrder(user)
			s.assignDeliveryPartner(order)
			s.Orders = append(s.Orders, *order)

			s.updateRestaurantMetrics(restaurant)
			nextOrderTime := s.generateNextOrderTime(user)
			s.EventQueue.Enqueue(&models.Event{
				Time: nextOrderTime,
				Type: models.EventPlaceOrder,
				Data: user,
			})
		}
	}
}

func (s *Simulator) generateOrdersWithProbability() {
	for _, user := range s.Users {
		// get user's segment
		segment := models.DefaultCustomerSegments[user.Segment]

		// calculate order probability based on segment and current time
		orderProbability := s.calculateOrderProbability(s.CurrentTime, segment)

		// adjust probability based on user's personal frequency
		orderProbability *= user.OrderFrequency / segment.OrdersPerMonth

		// determine if order should be placed
		if s.safeFloat64() < orderProbability {
			// create and process order
			order := s.createOrder(user)
			s.assignDeliveryPartner(order)
			s.Orders = append(s.Orders, *order)

			// select restaurant using reputation-based system
			nearbyRestaurants := s.getNearbyRestaurants(user.Location, 5.0)
			restaurant := s.updateOrderGeneration(user, nearbyRestaurants)
			if restaurant == nil {
				continue
			}
			s.updateRestaurantMetrics(restaurant)

			// update user's last order time
			user.LastOrderTime = s.CurrentTime

			// schedule next order
			nextOrderTime := s.generateNextOrderTime(user)
			s.EventQueue.Enqueue(&models.Event{
				Time: nextOrderTime,
				Type: models.EventPlaceOrder,
				Data: user,
			})

			// add event for order placement
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
	segment := models.DefaultCustomerSegments[user.Segment]
	restaurant := s.selectRestaurant(user)

	// select items considering user segment and preferences
	items := s.selectMenuItems(restaurant, user)

	// calculate base amount
	baseAmount := s.calculateTotalAmount(items)

	// adjust amount based on segment patterns
	adjustedAmount := baseAmount * (1 + (s.safeFloat64()*0.2 - 0.1)) // ±10% variation
	if segment.AvgSpend > 0 {
		// tendency towards segment's average spend
		adjustedAmount = (adjustedAmount + segment.AvgSpend) / 2
	}

	deliveryFee := s.calculateDeliveryFee(adjustedAmount)

	order := &models.Order{
		ID:            generateID(),
		CustomerID:    user.ID,
		RestaurantID:  restaurant.ID,
		Items:         items,
		TotalAmount:   adjustedAmount,
		DeliveryCost:  deliveryFee,
		OrderPlacedAt: s.CurrentTime,
		Status:        models.OrderStatusPlaced,
		PaymentMethod: s.generateRandomPaymentMethod(adjustedAmount),
	}

	// update user's order history
	user.LastOrderTime = s.CurrentTime
	user.LifetimeOrders++
	user.LifetimeSpend += adjustedAmount

	// update purchase patterns
	if user.PurchasePatterns == nil {
		user.PurchasePatterns = make(map[time.Weekday][]int)
	}
	weekday := s.CurrentTime.Weekday()
	hour := s.CurrentTime.Hour()
	user.PurchasePatterns[weekday] = append(user.PurchasePatterns[weekday], hour)

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
	order.CustomerID = user.ID

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
	for i := 0; i < len(s.Orders); i += batchSize {
		end := min(i+batchSize, len(s.Orders))
		s.processOrderBatch(s.Orders[i:end])
	}
}

func (s *Simulator) processOrderBatch(orders []models.Order) {
	for i, order := range orders {
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
	orderProbability := s.calculateTimeBasedOrderProbability(user, s.CurrentTime)

	// adjust based on time since last order
	if !user.LastOrderTime.IsZero() {
		hoursSinceLastOrder := s.CurrentTime.Sub(user.LastOrderTime).Hours()
		if hoursSinceLastOrder < 4 {
			orderProbability *= 0.1 // Significant reduction if ordered recently
		}
	}

	// special events multiplier
	if multiplier := s.getSpecialEventMultiplier(); multiplier > 1.0 {
		orderProbability *= multiplier
	}

	return s.safeFloat64() < orderProbability

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
	randomFactor := 0.8 + (0.4 * s.safeFloat64())
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

func (s *Simulator) assignPartnerToOrder(partner *models.DeliveryPartner, order *models.Order) error {
	// update order
	order.DeliveryPartnerID = partner.ID

	// update partner
	partner.Status = models.PartnerStatusEnRoutePickup
	partner.CurrentOrderID = order.ID

	// update the partner in the simulator's state
	partnerIndex := s.getPartnerIndex(partner.ID)
	if partnerIndex == -1 {
		return fmt.Errorf("partner not found in simulator state")
	}
	s.DeliveryPartners[partnerIndex] = partner

	// update the order in the simulator's state
	for i, o := range s.Orders {
		if o.ID == order.ID {
			s.Orders[i] = *order
			return nil
		}
	}

	return fmt.Errorf("order not found in simulator state")
}

func (s *Simulator) getPartnerCurrentOrder(partner *models.DeliveryPartner) *models.Order {
	if partner.CurrentOrderID == "" {
		return nil
	}

	// first, try to find the order by ID
	for i := len(s.Orders) - 1; i >= 0; i-- {
		order := &s.Orders[i]
		if order.ID == partner.CurrentOrderID {
			// Check if the order status is consistent with the partner having it
			if order.DeliveryPartnerID == partner.ID &&
				(order.Status == models.OrderStatusPickedUp ||
					order.Status == models.OrderStatusReady ||
					order.Status == models.OrderStatusInTransit) {
				return order
			} else {
				log.Printf("Warning: Inconsistent state for order %s and partner %s. Order status: %s, Partner status: %s",
					order.ID, partner.ID, order.Status, partner.Status)
				// consider correcting the inconsistency here
				return nil
			}
		}
	}

	// if not found by ID, look for any order assigned to this partner
	for i := len(s.Orders) - 1; i >= 0; i-- {
		order := &s.Orders[i]
		if order.DeliveryPartnerID == partner.ID &&
			(order.Status == models.OrderStatusPickedUp ||
				order.Status == models.OrderStatusReady ||
				order.Status == models.OrderStatusInTransit) {
			log.Printf("Warning: Partner %s has mismatched CurrentOrderID. Expected %s, found %s",
				partner.ID, partner.CurrentOrderID, order.ID)
			// consider updating partner.CurrentOrderID here
			return order
		}
	}

	log.Printf("Warning: Order %s not found for partner %s", partner.CurrentOrderID, partner.ID)
	// consider resetting partner.CurrentOrderID to "" here
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

func (s *Simulator) generateRandomPaymentMethod(orderAmount float64) string {
	// base weights
	weights := map[string]float64{
		"card":   0.5,
		"cash":   0.3,
		"wallet": 0.2,
	}

	// adjust weights based on order amount
	if orderAmount > 100 {
		// higher amounts are more likely to be paid by card
		weights["card"] += 0.2
		weights["cash"] -= 0.1
		weights["wallet"] -= 0.1
	} else if orderAmount < 20 {
		// lower amounts more likely to be cash
		weights["cash"] += 0.2
		weights["card"] -= 0.1
		weights["wallet"] -= 0.1
	}

	// normalise weights
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	for method := range weights {
		weights[method] /= total
	}

	// select payment method
	randVal := s.safeFloat64()
	cumulativeWeight := 0.0

	for method, weight := range weights {
		cumulativeWeight += weight
		if randVal <= cumulativeWeight {
			return method
		}
	}

	return "card" // default fallback
}

func (s *Simulator) calculateOrderProbability(currentTime time.Time, segment models.CustomerSegment) float64 {
	// Start with base probability from segment's orders per month
	baseProbability := segment.OrdersPerMonth / (30.0 * 24.0) // Convert to hourly probability

	// Apply time-based multipliers
	timeMultiplier := s.calculateTimeMultiplier(currentTime)
	weatherMultiplier := s.calculateWeatherOrderMultiplier()
	seasonalMultiplier := s.calculateSeasonalMultiplier(currentTime)
	eventMultiplier := s.calculateEventMultiplier(currentTime)

	probability := baseProbability * timeMultiplier * weatherMultiplier * seasonalMultiplier * eventMultiplier

	return math.Min(1.0, probability) // Cap at 100%
}

func (s *Simulator) calculateTimeMultiplier(currentTime time.Time) float64 {
	hour := currentTime.Hour()
	weekday := currentTime.Weekday()

	// Base multiplier
	multiplier := 1.0

	// Time of day adjustments
	switch {
	case hour >= 11 && hour <= 14: // Lunch rush
		multiplier *= 2.0
	case hour >= 18 && hour <= 21: // Dinner rush
		multiplier *= 2.5
	case hour >= 22 || hour <= 5: // Late night/early morning
		multiplier *= 0.3
	case hour >= 6 && hour <= 10: // Breakfast
		multiplier *= 1.2
	case hour >= 15 && hour <= 17: // Afternoon lull
		multiplier *= 0.7
	}

	// Day of week adjustments
	switch weekday {
	case time.Friday:
		multiplier *= 1.4
		// Additional boost for Friday dinner and late night
		if hour >= 17 {
			multiplier *= 1.3
		}
	case time.Saturday:
		multiplier *= 1.5
		// Weekend specific timing adjustments
		if hour >= 11 && hour <= 14 {
			multiplier *= 1.2 // Longer lunch period
		}
	case time.Sunday:
		multiplier *= 1.3
		// Sunday specific patterns
		if hour >= 10 && hour <= 15 {
			multiplier *= 1.4 // Brunch hours
		}
	case time.Monday:
		multiplier *= 0.9 // Generally slower
	case time.Thursday:
		multiplier *= 1.1 // Slight end-of-week increase
	}

	// pay period effect (assuming bi-monthly)
	dayOfMonth := currentTime.Day()
	if dayOfMonth == 15 || dayOfMonth <= 2 {
		multiplier *= 1.3 // payday boost
	}

	return multiplier
}

func (s *Simulator) calculateWeatherOrderMultiplier() float64 {
	weather := s.getCurrentWeather()
	temp := s.getCurrentTemperature()

	// base multiplier
	multiplier := 1.0

	// weather condition adjustments
	weatherMultipliers := map[string]float64{
		"rain":   1.4, // People more likely to order in rain
		"snow":   1.6, // Even more likely in snow
		"storm":  1.8, // Highest during storms
		"cloudy": 1.1, // Slight increase
		"clear":  0.9, // Slight decrease in good weather
		"fog":    1.2, // Moderate increase
		"windy":  1.2,
	}

	if multiplier, exists := weatherMultipliers[weather]; exists {
		multiplier *= multiplier
	}

	// temperature adjustments
	switch {
	case temp <= 0:
		multiplier *= 1.5 // cold weather increases orders
	case temp <= 10:
		multiplier *= 1.3
	case temp >= 30:
		multiplier *= 1.4 // hot weather increases orders
	case temp >= 25:
		multiplier *= 1.2
	case temp >= 15 && temp <= 25:
		multiplier *= 0.9 // pleasant weather decreases orders
	}

	// time-based weather impact
	hour := s.CurrentTime.Hour()
	if hour >= 17 && hour <= 22 { // evening hours
		if weather == "rain" || weather == "snow" || weather == "storm" {
			multiplier *= 1.2 // weather has stronger effect during dinner hours
		}
	}

	return multiplier
}

func (s *Simulator) calculateSeasonalMultiplier(currentTime time.Time) float64 {
	month := currentTime.Month()

	// base seasonal patterns
	seasonalMultipliers := map[time.Month]float64{
		time.January:   1.2, // Post-holiday, cold weather
		time.February:  1.1,
		time.March:     1.0,
		time.April:     0.9,
		time.May:       0.8, // Nice weather, less ordering
		time.June:      0.9,
		time.July:      1.0, // Hot weather increases orders
		time.August:    1.0,
		time.September: 1.1, // Back to school/work
		time.October:   1.2,
		time.November:  1.3, // Cold weather, holiday season
		time.December:  1.4, // Peak holiday season
	}

	multiplier := seasonalMultipliers[month]

	// adjust for specific periods
	dayOfMonth := currentTime.Day()

	// holiday season adjustments (December)
	if month == time.December {
		if dayOfMonth >= 15 && dayOfMonth <= 24 {
			multiplier *= 1.3 // Pre-Christmas boost
		} else if dayOfMonth >= 26 && dayOfMonth <= 31 {
			multiplier *= 1.2 // Post-Christmas/New Year
		}
	}

	// summer vacation effect
	if (month == time.July || month == time.August) &&
		(dayOfMonth >= 15 && dayOfMonth <= 31) {
		multiplier *= 0.9 // Reduced orders during peak vacation
	}

	// beginning of school year
	if month == time.September && dayOfMonth <= 15 {
		multiplier *= 1.2 // back to school boost
	}

	return multiplier
}

func (s *Simulator) calculateEventMultiplier(currentTime time.Time) float64 {
	multiplier := 1.0

	// check for special dates
	dateKey := currentTime.Format("01-02")
	specialDates := map[string]float64{
		"12-31": 2.0, // New Year's Eve
		"12-24": 1.5, // Christmas Eve
		"12-25": 0.3, // Christmas Day
		"01-01": 1.7, // New Year's Day
		"02-14": 1.8, // Valentine's Day
		"10-31": 1.4, // Halloween
	}

	if eventMultiplier, exists := specialDates[dateKey]; exists {
		multiplier *= eventMultiplier
	}

	// Academic calendar effects
	if s.isUniversityArea() {
		switch {
		case s.isFinalsWeek():
			multiplier *= 1.5 // Students ordering more during finals
		case s.isMovingWeek():
			multiplier *= 1.3 // High activity during moving periods
		case s.isSpringBreak():
			multiplier *= 0.7 // Lower activity during breaks
		}
	}

	// cultural and local festivals
	if events := s.getLocalFestivals(currentTime); len(events) > 0 {
		// different festivals might have different effects
		for _, festival := range events {
			multiplier *= festival.Multiplier
		}
	}

	return multiplier
}

func (s *Simulator) getLocalFestivals(t time.Time) []LocalEvent {
	var festivals []LocalEvent

	// Example festival definitions
	// In a real implementation, this would likely come from a database or configuration
	yearDay := t.YearDay()

	// Example: Summer Festival (June 1-7)
	if month := t.Month(); month == time.June && t.Day() <= 7 {
		festivals = append(festivals, LocalEvent{
			Type:       "festival",
			Multiplier: 0.8, // Reduced delivery orders during festival
			StartTime:  time.Date(t.Year(), time.June, 1, 0, 0, 0, 0, t.Location()),
			EndTime:    time.Date(t.Year(), time.June, 7, 23, 59, 59, 0, t.Location()),
		})
	}

	// Example: Food Truck Week (reduces restaurant orders)
	if weekNum := yearDay / 7; weekNum == 30 { // Around late July
		festivals = append(festivals, LocalEvent{
			Type:       "food_event",
			Multiplier: 0.7,
			StartTime:  t,
			EndTime:    t.AddDate(0, 0, 7),
		})
	}

	return festivals
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
		selectedPartner := availablePartners[s.safeIntn(len(availablePartners))]
		if selectedPartner != nil {
			order.DeliveryPartnerID = selectedPartner.ID
			selectedPartner.Status = models.PartnerStatusEnRoutePickup
			selectedPartner.CurrentOrderID = order.ID
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
		log.Printf("Partner %s status: %s, isNear: %v, distance: %.2f km",
			partner.ID, partner.Status, isNear,
			s.calculateDistance(partner.CurrentLocation, location))
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

func (s *Simulator) updatePartnerMetrics(partner *models.DeliveryPartner, newLocation models.Location, currentSpeed float64) {
	// calculate distance traveled
	distance := s.calculateDistance(partner.CurrentLocation, newLocation)

	// update experience based on distance (very small increment per km)
	partner.Experience += distance * 0.0001

	// update average speed (moving average)
	partner.AvgSpeed = partner.AvgSpeed*0.95 + currentSpeed*0.05

	// update location and time
	partner.CurrentLocation = newLocation
	partner.Speed = currentSpeed
	partner.LastUpdateTime = s.CurrentTime
}

func (s *Simulator) updateDeliveryPartnerLocations() {
	for i, partner := range s.DeliveryPartners {
		if partner.Status == models.PartnerStatusAvailable {
			continue
		}

		order := s.getPartnerCurrentOrder(partner)
		if order == nil {
			continue
		}

		var destination models.Location
		if partner.Status == models.PartnerStatusEnRoutePickup {
			restaurant := s.getRestaurant(order.RestaurantID)
			if restaurant == nil {
				continue
			}
			destination = restaurant.Location
		} else if partner.Status == models.PartnerStatusEnRouteDelivery {
			user := s.getUser(order.CustomerID)
			if user == nil {
				continue
			}
			destination = user.Location
		} else {
			continue
		}

		// Move partner towards destination
		duration := s.CurrentTime.Sub(partner.LastUpdateTime)
		newLocation := s.moveTowards(partner.CurrentLocation, destination, duration)
		s.DeliveryPartners[i].CurrentLocation = newLocation
		s.DeliveryPartners[i].LastUpdateTime = s.CurrentTime

		// Check if destination reached
		if s.isAtLocation(newLocation, destination) {
			if partner.Status == models.PartnerStatusEnRoutePickup {
				// Schedule pickup
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventPickUpOrder,
					Data: order,
				})
			} else if partner.Status == models.PartnerStatusEnRouteDelivery {
				// Schedule delivery
				s.EventQueue.Enqueue(&models.Event{
					Time: s.CurrentTime,
					Type: models.EventDeliverOrder,
					Data: order,
				})
			}
		}
	}
}

func (s *Simulator) estimateArrivalTime(from, to models.Location) time.Time {
	distance := s.calculateDistance(from, to)
	travelTime := distance / s.Config.PartnerMoveSpeed // Assuming PartnerMoveSpeed is in km/hour

	// add some variability to the travel time
	variability := 0.2 // 20% variability
	actualTravelTime := travelTime * (1 + (s.safeFloat64()*2-1)*variability)

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

	// calculate base travel times
	timeToRestaurant := s.calculateTravelTime(partner.CurrentLocation, restaurant.Location)
	timeToCustomer := s.calculateTravelTime(restaurant.Location, user.Location)

	// get current traffic and weather conditions
	trafficMultiplier := s.getCurrentTrafficMultiplier()
	weatherMultiplier := s.getCurrentWeatherMultiplier()

	// apply time-based adjustments
	timeMultiplier := s.getTimeBasedDeliveryMultiplier()

	// Calculate total travel time with all factors
	totalTravelTime := (timeToRestaurant + timeToCustomer) *
		trafficMultiplier *
		weatherMultiplier *
		timeMultiplier

	// add buffer time for various activities
	bufferTime := s.calculateBufferTime(partner, order)

	// add some randomness (±10%)
	randomFactor := 0.9 + (s.safeFloat64() * 0.2)

	totalTime := totalTravelTime*randomFactor + bufferTime

	return s.CurrentTime.Add(time.Duration(totalTime) * time.Minute)
}

func (s *Simulator) calculateTravelTime(from, to models.Location) float64 {
	distance := s.calculateDistance(from, to)
	baseSpeed := 30.0 // km/h

	// adjust speed based on area type
	if s.isUrbanArea(from) && s.isUrbanArea(to) {
		baseSpeed = 25.0 // slower in urban areas
	} else if !s.isUrbanArea(from) && !s.isUrbanArea(to) {
		baseSpeed = 40.0 // faster in suburban areas
	}

	return (distance / baseSpeed) * 60 // Convert to minutes
}

func (s *Simulator) getCurrentTrafficMultiplier() float64 {
	hour := s.CurrentTime.Hour()
	weekday := s.CurrentTime.Weekday()

	// base multiplier
	multiplier := 1.0

	// peak hour traffic
	if (hour >= 7 && hour <= 9) || (hour >= 16 && hour <= 18) {
		multiplier *= 1.5
		if weekday >= time.Monday && weekday <= time.Friday {
			multiplier *= 1.2 // even worse during weekday rush hours
		}
	}

	// late night traffic
	if hour >= 22 || hour <= 5 {
		multiplier *= 0.8 // less traffic at night
	}

	return multiplier
}

func (s *Simulator) calculateBufferTime(partner *models.DeliveryPartner, order *models.Order) float64 {
	baseBuffer := 5.0 // base 5 minutes

	// add buffer based on partner experience
	if partner.Experience < 0.3 {
		baseBuffer += 3.0 // less experienced drivers need more buffer
	}

	// add buffer based on order size
	if len(order.Items) > 3 {
		baseBuffer += float64(len(order.Items)) * 0.5
	}

	// add buffer based on restaurant efficiency
	restaurant := s.getRestaurant(order.RestaurantID)
	if restaurant != nil && restaurant.PickupEfficiency < 0.8 {
		baseBuffer += 5.0 // add more buffer for less efficient restaurants
	}

	return baseBuffer
}

func (s *Simulator) getTimeBasedDeliveryMultiplier() float64 {
	hour := s.CurrentTime.Hour()
	multiplier := 1.0

	switch {
	case hour >= 22 || hour <= 4: // late night
		multiplier = 0.9 // faster deliveries due to less traffic
	case hour >= 7 && hour <= 9: // morning rush
		multiplier = 1.4
	case hour >= 16 && hour <= 18: // evening rush
		multiplier = 1.5
	case hour >= 11 && hour <= 13: // lunch rush
		multiplier = 1.3
	}

	if s.CurrentTime.Weekday() == time.Friday {
		if hour >= 17 && hour <= 23 {
			multiplier *= 1.2 // Friday nights are busier
		}
	}

	return multiplier
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
	currentPattern := s.getCurrentOrderPattern(s.CurrentTime)
	selectedItems := make([]string, 0)

	// determine number of items based on user segment and time
	itemCount := s.determineOrderSize(user, currentPattern)

	// group menu items by type
	itemsByType := make(map[string][]*models.MenuItem)
	for _, itemID := range restaurant.MenuItems {
		if item := s.getMenuItem(itemID); item != nil {
			itemsByType[item.Type] = append(itemsByType[item.Type], item)
		}
	}

	// select items based on pattern preferences
	for itemType, multiplier := range currentPattern.MenuPreferences {
		if items, exists := itemsByType[itemType]; exists {
			// calculate probabilities for each item
			probabilities := make([]float64, len(items))
			totalProb := 0.0

			for i, item := range items {
				prob := s.calculateMenuItemProbability(item, s.CurrentTime)
				prob *= multiplier // apply pattern multiplier

				// apply user preferences
				if s.matchesUserPreferences(item, user) {
					prob *= 1.5
				}

				probabilities[i] = prob
				totalProb += prob
			}

			// select items probabilistically
			if totalProb > 0 {
				selected := s.selectProbabilistically(items, probabilities)
				if selected != nil {
					selectedItems = append(selectedItems, selected.ID)
				}
			}
		}
	}

	// fill remaining slots with random items if needed
	for len(selectedItems) < itemCount {
		if randomItem := s.selectRandomMenuItem(restaurant, selectedItems); randomItem != nil {
			selectedItems = append(selectedItems, randomItem.ID)
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

func (s *Simulator) selectRandomMenuItem(restaurant *models.Restaurant, existingItems []string) *models.MenuItem {
	if restaurant == nil || len(restaurant.MenuItems) == 0 {
		return nil
	}

	// create a map of already selected items for quick lookup
	selectedItems := make(map[string]bool)
	for _, itemID := range existingItems {
		selectedItems[itemID] = true
	}

	// get all eligible items and their probabilities
	var eligibleItems []*models.MenuItem
	var probabilities []float64
	totalProbability := 0.0

	for _, itemID := range restaurant.MenuItems {
		// skip if already selected
		if selectedItems[itemID] {
			continue
		}

		item := s.getMenuItem(itemID)
		if item == nil {
			continue
		}

		// calculate selection probability based on multiple factors
		probability := s.calculateItemSelectionProbability(item, existingItems)
		if probability > 0 {
			eligibleItems = append(eligibleItems, item)
			probabilities = append(probabilities, probability)
			totalProbability += probability
		}
	}

	if len(eligibleItems) == 0 {
		return nil
	}

	// normalise probabilities
	for i := range probabilities {
		probabilities[i] /= totalProbability
	}

	// select item based on probabilities
	randVal := s.safeFloat64()
	cumulativeProbability := 0.0

	for i, item := range eligibleItems {
		cumulativeProbability += probabilities[i]
		if randVal <= cumulativeProbability {
			return item
		}
	}

	// default to last item if something goes wrong with probabilities
	return eligibleItems[len(eligibleItems)-1]
}

func (s *Simulator) calculateItemSelectionProbability(item *models.MenuItem, existingItems []string) float64 {
	if item == nil {
		return 0
	}

	// start with base popularity
	probability := item.Popularity

	// time-based adjustments
	timeMultiplier := s.getTimeBasedItemMultiplier(item)
	probability *= timeMultiplier

	// weather-based adjustments
	weatherMultiplier := s.getWeatherBasedItemMultiplier(item)
	probability *= weatherMultiplier

	// complementary items adjustment
	complementaryMultiplier := s.getComplementaryItemMultiplier(item, existingItems)
	probability *= complementaryMultiplier

	// seasonal adjustment
	seasonalMultiplier := s.getSeasonalItemMultiplier(item)
	probability *= seasonalMultiplier

	// price sensitivity adjustment (lower probability for very expensive items)
	priceFactor := s.getPriceBasedMultiplier(item)
	probability *= priceFactor

	return probability
}

func (s *Simulator) getCurrentWeatherMultiplier() float64 {
	// start with base multiplier
	multiplier := 1.0

	weather := s.getCurrentWeather()
	temp := s.getCurrentTemperature()
	hour := s.CurrentTime.Hour()

	// base weather condition multipliers
	weatherMultipliers := map[string]struct {
		capacity float64 // effect on restaurant capacity
		delivery float64 // effect on delivery operations
		duration float64 // how long this effect typically lasts
		severity float64 // how much it impacts overall operations
	}{
		"rain": {
			capacity: 1.3, // people more likely to order in
			delivery: 0.8, // slower/more careful delivery
			duration: 1.0, // normal duration impact
			severity: 1.0, // normal severity
		},
		"heavy_rain": {
			capacity: 1.4, // even more likely to order in
			delivery: 0.6, // much slower delivery
			duration: 1.2, // longer impact
			severity: 1.3, // more severe
		},
		"storm": {
			capacity: 1.5, // highest likelihood of ordering in
			delivery: 0.5, // significant delivery slowdown
			duration: 1.5, // extended impact
			severity: 1.5, // most severe
		},
		"snow": {
			capacity: 1.4, // high likelihood of ordering in
			delivery: 0.6, // significant slowdown
			duration: 1.3, // extended impact
			severity: 1.4, // very severe
		},
		"fog": {
			capacity: 1.1, // slight increase in orders
			delivery: 0.8, // moderate slowdown
			duration: 0.8, // shorter impact
			severity: 0.9, // less severe
		},
		"clear": {
			capacity: 0.9, // people more likely to go out
			delivery: 1.1, // faster delivery possible
			duration: 1.0, // normal duration
			severity: 0.8, // minimal impact
		},
		"cloudy": {
			capacity: 1.0, // normal ordering patterns
			delivery: 1.0, // normal delivery speed
			duration: 1.0, // normal duration
			severity: 0.9, // minor impact
		},
		"windy": {
			capacity: 1.2, // slightly more likely to order in
			delivery: 0.9, // slight delivery slowdown
			severity: 1.1, // moderate impact
			duration: 0.9, // shorter impact
		},
	}

	// get base multipliers for current weather
	if conditions, exists := weatherMultipliers[weather]; exists {
		// start with the capacity multiplier as base
		multiplier = conditions.capacity

		// adjust for time of day
		if hour >= 17 && hour <= 22 { // Evening hours
			// weather has stronger effect during dinner hours
			multiplier *= 1.1
		}

		// adjust for temperature
		switch {
		case temp <= 0:
			// cold weather increases impact
			multiplier *= 1.2
		case temp >= 30:
			// hot weather increases impact
			multiplier *= 1.15
		case temp >= 15 && temp <= 25:
			// pleasant temperature reduces weather impact
			multiplier *= 0.9
		}

		// consider weather severity
		multiplier *= conditions.severity

		// consider duration of weather condition
		if s.WeatherState != nil {
			elapsedDuration := s.CurrentTime.Sub(s.WeatherState.StartTime)
			if elapsedDuration > 6*time.Hour {
				// long-duration weather has diminishing effect
				multiplier *= 0.9
			}
		}
	}

	// special case combinations
	if weather == "rain" && hour >= 17 && hour <= 21 {
		// rainy dinner hours have stronger effect
		multiplier *= 1.2
	}

	if weather == "snow" && s.isWeekend(s.CurrentTime) {
		// snow has stronger effect on weekends
		multiplier *= 1.2
	}

	// temperature extremes adjustment
	temperatureMultiplier := s.getTemperatureExtremesMultiplier(temp)
	multiplier *= temperatureMultiplier

	// urban density impact
	if s.isUrbanArea(s.getCurrentLocation()) {
		// weather has stronger effect in urban areas due to higher delivery density
		multiplier *= 1.1
	}

	// holiday adjustment
	if s.isHoliday() {
		// weather has stronger effect during holidays
		multiplier *= 1.2
	}

	// cap the multiplier to reasonable bounds
	multiplier = math.Max(0.5, math.Min(2.0, multiplier))

	return multiplier
}

func (s *Simulator) getTemperatureExtremesMultiplier(temp float64) float64 {
	multiplier := 1.0

	switch {
	case temp <= -10:
		// extreme cold
		multiplier = 1.5
	case temp <= 0:
		// cold
		multiplier = 1.3
	case temp >= 35:
		// extreme heat
		multiplier = 1.4
	case temp >= 30:
		// hot
		multiplier = 1.2
	case temp >= 15 && temp <= 25:
		// pleasant weather
		multiplier = 0.9
	}

	return multiplier
}

func (s *Simulator) getCurrentLocation() models.Location {
	return models.Location{
		Lat: s.Config.CityLat,
		Lon: s.Config.CityLon,
	}
}

func (s *Simulator) getTimeBasedItemMultiplier(item *models.MenuItem) float64 {
	hour := s.CurrentTime.Hour()
	multiplier := 1.0

	switch item.Type {
	case "breakfast":
		if hour >= 6 && hour <= 11 {
			multiplier = 2.0
		} else {
			multiplier = 0.2 // much less likely outside breakfast hours
		}

	case "lunch":
		if hour >= 11 && hour <= 15 {
			multiplier = 1.8
		}

	case "dinner":
		if hour >= 17 && hour <= 22 {
			multiplier = 1.8
		}

	case "dessert":
		if hour >= 11 && hour <= 23 {
			multiplier = 1.2
			// higher probability after meal times
			if hour >= 13 && hour <= 15 || hour >= 19 && hour <= 22 {
				multiplier = 1.5
			}
		}

	case "drink":
		// higher probability during meal times and late evening
		if hour >= 11 && hour <= 14 || hour >= 18 && hour <= 23 {
			multiplier = 1.3
		}
		// even higher for cold drinks during hot hours
		if s.getCurrentTemperature() > 25 && hour >= 12 && hour <= 18 {
			multiplier *= 1.4
		}
	}

	return multiplier
}

func (s *Simulator) getWeatherBasedItemMultiplier(item *models.MenuItem) float64 {
	multiplier := 1.0
	weather := s.getCurrentWeather()
	temp := s.getCurrentTemperature()

	// temperature-based adjustments
	if temp > 25 {
		// hot weather preferences
		if strings.Contains(strings.ToLower(item.Name), "cold") ||
			strings.Contains(strings.ToLower(item.Name), "ice") ||
			strings.Contains(strings.ToLower(item.Name), "salad") {
			multiplier *= 1.5
		}
		if strings.Contains(strings.ToLower(item.Name), "hot") ||
			strings.Contains(strings.ToLower(item.Name), "soup") {
			multiplier *= 0.7
		}
	} else if temp < 10 {
		// cold weather preferences
		if strings.Contains(strings.ToLower(item.Name), "soup") ||
			strings.Contains(strings.ToLower(item.Name), "hot") ||
			strings.Contains(strings.ToLower(item.Name), "warm") {
			multiplier *= 1.5
		}
		if strings.Contains(strings.ToLower(item.Name), "ice") ||
			strings.Contains(strings.ToLower(item.Name), "cold") {
			multiplier *= 0.7
		}
	}

	// weather condition adjustments
	switch weather {
	case "rain", "snow":
		if strings.Contains(strings.ToLower(item.Name), "soup") ||
			strings.Contains(strings.ToLower(item.Name), "hot") {
			multiplier *= 1.3
		}
	case "clear":
		if strings.Contains(strings.ToLower(item.Name), "fresh") ||
			strings.Contains(strings.ToLower(item.Name), "salad") {
			multiplier *= 1.2
		}
	}

	return multiplier
}

func (s *Simulator) getComplementaryItemMultiplier(item *models.MenuItem, existingItems []string) float64 {
	if len(existingItems) == 0 {
		return 1.0
	}

	multiplier := 1.0
	hasMainCourse := false
	hasDrink := false
	hasAppetizer := false
	hasDessert := false

	// check existing items
	for _, itemID := range existingItems {
		existingItem := s.getMenuItem(itemID)
		if existingItem == nil {
			continue
		}

		switch existingItem.Type {
		case "main course":
			hasMainCourse = true
		case "drink":
			hasDrink = true
		case "appetizer":
			hasAppetizer = true
		case "dessert":
			hasDessert = true
		}
	}

	// adjust probability based on meal composition
	switch item.Type {
	case "main course":
		if hasMainCourse {
			multiplier *= 0.3 // less likely to order multiple main courses
		}
	case "drink":
		if !hasDrink {
			multiplier *= 1.5 // more likely to add a drink if none ordered
		}
	case "appetizer":
		if !hasAppetizer && !hasDessert {
			multiplier *= 1.3 // more likely to add appetizer if no appetizer/dessert
		}
	case "dessert":
		if hasMainCourse && !hasDessert {
			multiplier *= 1.4 // more likely to add dessert after main course
		}
	case "side dish":
		if hasMainCourse {
			multiplier *= 1.5 // more likely to add sides with main course
		}
	}

	return multiplier
}

func (s *Simulator) getSeasonalItemMultiplier(item *models.MenuItem) float64 {
	season := s.getCurrentSeason()
	multiplier := 1.0

	// seasonal adjustments based on item name and ingredients
	switch season {
	case "summer":
		if containsAny(item.Name, []string{"fresh", "light", "salad", "cold", "ice"}) {
			multiplier *= 1.4
		}
	case "winter":
		if containsAny(item.Name, []string{"hot", "warm", "soup", "stew", "roast"}) {
			multiplier *= 1.4
		}
	case "spring":
		if containsAny(item.Name, []string{"fresh", "spring", "light"}) {
			multiplier *= 1.3
		}
	case "fall":
		if containsAny(item.Name, []string{"pumpkin", "autumn", "harvest"}) {
			multiplier *= 1.3
		}
	}

	return multiplier
}

func (s *Simulator) getPriceBasedMultiplier(item *models.MenuItem) float64 {
	// base multiplier starts at 1
	multiplier := 1.0

	// get average price for this type of item
	avgPrice := s.getAveragePriceForItemType(item.Type)
	if avgPrice <= 0 {
		return multiplier
	}

	// calculate price ratio
	priceRatio := item.Price / avgPrice

	// adjust multiplier based on price ratio
	if priceRatio > 1.5 {
		// expensive items are less likely to be randomly selected
		multiplier *= 0.7
	} else if priceRatio < 0.8 {
		// cheaper items are more likely to be randomly selected
		multiplier *= 1.2
	}

	return multiplier
}

func (s *Simulator) getAveragePriceForItemType(itemType string) float64 {
	var totalPrice float64
	var count int

	// calculate average price for this type of item across all restaurants
	for _, restaurant := range s.Restaurants {
		for _, itemID := range restaurant.MenuItems {
			item := s.getMenuItem(itemID)
			if item != nil && item.Type == itemType {
				totalPrice += item.Price
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return totalPrice / float64(count)
}

func (s *Simulator) calculateMenuItemProbability(item *models.MenuItem, currentTime time.Time) float64 {
	baseProb := item.Popularity

	// get time pattern for item type
	if pattern, exists := MenuTimePatterns[item.Type]; exists {
		// check if current hour is peak hour
		currentHour := currentTime.Hour()
		for _, peakHour := range pattern.PeakHours {
			if currentHour == peakHour {
				baseProb *= 1.5
				break
			}
		}

		// check seasonal preference
		currentMonth := currentTime.Month()
		for _, seasonalMonth := range pattern.SeasonalMonths {
			if currentMonth == seasonalMonth {
				baseProb *= 1.3
				break
			}
		}

		// apply day part preference
		dayPart := getDayPart(currentHour)
		if multiplier, exists := pattern.DayPartPreference[dayPart]; exists {
			baseProb *= multiplier
		}

		// apply weather preference
		if multiplier, exists := pattern.WeatherPreference[s.getCurrentWeather()]; exists {
			baseProb *= multiplier
		}
	}

	return baseProb
}

func (s *Simulator) getCurrentOrderPattern(t time.Time) OrderPattern {
	hour := t.Hour()

	// find the most appropriate pattern for current time
	var currentPattern OrderPattern
	highestMultiplier := 0.0

	for _, pattern := range DefaultOrderPatterns {
		if multiplier, exists := pattern.TimeMultipliers[hour]; exists {
			if multiplier > highestMultiplier {
				highestMultiplier = multiplier
				currentPattern = pattern
			}
		}
	}

	// if no specific pattern found, return default pattern
	if highestMultiplier == 0 {
		return DefaultOrderPatterns["normal"]
	}

	return currentPattern
}

func (s *Simulator) determineOrderSize(user *models.User, pattern OrderPattern) int {
	segment := models.DefaultCustomerSegments[user.Segment]

	// base size based on segment
	baseSize := 2
	if segment.AvgSpend > 40 {
		baseSize = 3
	}

	// adjust for time pattern
	if pattern.Type == "dinner_rush" {
		baseSize++
	}

	// random variation
	variation := s.safeIntn(2) - 1 // -1, 0, or 1

	return max(1, baseSize+variation)
}

func (s *Simulator) matchesUserPreferences(item *models.MenuItem, user *models.User) bool {
	// check dietary restrictions
	for _, restriction := range user.DietaryRestrictions {
		for _, ingredient := range item.Ingredients {
			if strings.EqualFold(restriction, ingredient) {
				return false
			}
		}
	}

	// check preferred cuisines
	for _, preference := range user.Preferences {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(preference)) {
			return true
		}
	}

	return false
}

func (s *Simulator) selectProbabilistically(items []*models.MenuItem, probabilities []float64) *models.MenuItem {
	if len(items) == 0 {
		return nil
	}

	totalProb := 0.0
	for _, prob := range probabilities {
		totalProb += prob
	}

	if totalProb == 0 {
		return items[s.safeIntn(len(items))]
	}

	r := s.safeFloat64() * totalProb
	cumulative := 0.0

	for i, item := range items {
		cumulative += probabilities[i]
		if r <= cumulative {
			return item
		}
	}

	return items[len(items)-1]
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
			continue // skip if item not found
		}

		subtotal += item.Price

		// add to discountable total if the item is eligible for discounts
		if item.IsDiscountEligible {
			discountableTotal += item.Price
		}
	}

	// calculate discount
	var discountAmount float64
	if discountableTotal >= s.Config.MinOrderForDiscount {
		discountAmount = discountableTotal * s.Config.DiscountPercentage
		if discountAmount > s.Config.MaxDiscountAmount {
			discountAmount = s.Config.MaxDiscountAmount
		}
	}

	// calculate tax
	taxAmount := subtotal * s.Config.TaxRate

	// calculate delivery fee (if applicable)
	deliveryFee := s.calculateDeliveryFee(subtotal)

	// calculate service fee
	serviceFee := subtotal * s.Config.ServiceFeePercentage

	// calculate total
	total := subtotal + taxAmount + deliveryFee + serviceFee - discountAmount

	// round to two decimal places
	return math.Round(total*100) / 100
}

func (s *Simulator) calculateDeliveryFee(subtotal float64) float64 {
	if subtotal >= s.Config.FreeDeliveryThreshold {
		return 0
	}

	// base delivery fee
	fee := s.Config.BaseDeliveryFee

	// additional fee for small orders
	if subtotal < s.Config.SmallOrderThreshold {
		fee += s.Config.SmallOrderFee
	}

	return fee
}

func (s *Simulator) updateRestaurantMetrics(restaurant *models.Restaurant) {
	// update average prep time
	totalPrepTime := 0.0
	for _, order := range restaurant.CurrentOrders {
		if order.PrepStartTime.After(time.Time{}) && order.PickupTime.After(time.Time{}) {
			totalPrepTime += order.PickupTime.Sub(order.PrepStartTime).Minutes()
		}
	}
	if len(restaurant.CurrentOrders) > 0 {
		restaurant.AvgPrepTime = totalPrepTime / float64(len(restaurant.CurrentOrders))
	}

	// update restaurant efficiency
	restaurant.PickupEfficiency = s.adjustPickupEfficiency(restaurant)

	// update restaurant capacity
	restaurant.Capacity = int(float64(restaurant.Capacity) * restaurant.PickupEfficiency)
}

func (s *Simulator) updateRestaurantOrderMetrics(restaurantID string, metrics models.OrderMetrics) {
	restaurant := s.getRestaurant(restaurantID)
	if restaurant == nil {
		return
	}

	// update restaurant efficiency metrics
	if metrics.TotalOrders > 0 {
		// update average preparation time
		if metrics.AvgPrepTime > 0 {
			restaurant.AvgPrepTime = restaurant.AvgPrepTime*0.7 + metrics.AvgPrepTime*0.3
		}

		// update pickup efficiency based on completion rate and timing
		newEfficiency := metrics.CompletionRate *
			(1.0 - float64(metrics.LateDeliveries)/float64(metrics.TotalOrders))
		restaurant.PickupEfficiency = restaurant.PickupEfficiency*0.8 + newEfficiency*0.2

		// adjust capacity based on peak hour performance
		maxOrdersPerHour := 0
		for _, count := range metrics.PeakHours {
			if count > maxOrdersPerHour {
				maxOrdersPerHour = count
			}
		}
		if maxOrdersPerHour > restaurant.Capacity {
			// gradually increase capacity if consistently handling more orders
			restaurant.Capacity = int(float64(restaurant.Capacity) * 1.1)
		} else if maxOrdersPerHour < restaurant.Capacity/2 {
			// gradually decrease capacity if consistently underutilized
			restaurant.Capacity = int(float64(restaurant.Capacity) * 0.95)
		}
	}

	// update menu item popularity
	for itemID, orderCount := range metrics.PopularItems {
		if item := s.getMenuItem(itemID); item != nil {
			// update item popularity based on order frequency
			newPopularity := float64(orderCount) / float64(metrics.TotalOrders)
			item.Popularity = item.Popularity*0.8 + newPopularity*0.2
		}
	}

	// cache the metrics for future use
	s.updateRestaurantPerformanceCache(restaurantID, metrics)
}

func (s *Simulator) updateRestaurantPerformanceCache(restaurantID string, metrics models.OrderMetrics) {
	// sort peak hours
	var peakHours []int
	for hour, count := range metrics.PeakHours {
		if count >= metrics.TotalOrders/24 {
			peakHours = append(peakHours, hour)
		}
	}
	sort.Ints(peakHours)

	// sort popular items
	type itemPopularity struct {
		ID    string
		Count int
	}
	var popularItems []models.ItemPopularity
	for itemID, count := range metrics.PopularItems {
		popularItems = append(popularItems, models.ItemPopularity{itemID, count})
	}
	sort.Slice(popularItems, func(i, j int) bool {
		return popularItems[i].Count > popularItems[j].Count
	})

	// update cache
	s.RestaurantPerformanceCache[restaurantID] = &models.RestaurantPerformanceCache{
		HourlyOrderCounts: metrics.PeakHours,
		PeakHours:         peakHours,
		PopularItems:      popularItems,
		RecentMetrics:     metrics,
		LastUpdate:        s.CurrentTime,
	}
}

func (s *Simulator) adjustRestaurantCapacity(restaurant *models.Restaurant) int {
	cluster := s.getRestaurantCluster(restaurant)
	baseCapacity := cluster.BaseCapacity

	// time-based adjustments
	timeMultiplier := s.getTimeBasedCapacityMultiplier(s.CurrentTime)

	// weather impact
	weatherMultiplier := s.getWeatherCapacityMultiplier()

	// special events impact
	eventMultiplier := s.getSpecialEventCapacityMultiplier()

	// staff efficiency impact (could be based on historical performance)
	efficiencyMultiplier := restaurant.PickupEfficiency

	// calculate adjusted capacity
	adjustedCapacity := float64(baseCapacity) * timeMultiplier * weatherMultiplier *
		eventMultiplier * efficiencyMultiplier

	// add random variation within cluster flexibility
	flexibility := cluster.CapacityFlexibility
	variation := (s.safeFloat64()*2 - 1) * flexibility
	adjustedCapacity *= 1 + variation

	return int(math.Max(float64(cluster.BaseCapacity/2),
		math.Min(float64(cluster.BaseCapacity*2), adjustedCapacity)))
}

func (s *Simulator) getRestaurantCluster(restaurant *models.Restaurant) RestaurantCluster {
	avgPrice := s.calculateAverageItemPrice(restaurant)
	if avgPrice > 30 {
		return RestaurantClusters["premium"]
	} else if avgPrice > 15 {
		return RestaurantClusters["standard"]
	}
	return RestaurantClusters["budget"]
}

func (s *Simulator) getRestaurantPattern(restaurant *models.Restaurant) RestaurantPattern {
	for _, cuisine := range restaurant.Cuisines {
		if pattern, exists := RestaurantPatterns[strings.ToLower(cuisine)]; exists {
			return pattern
		}
	}
	return RestaurantPatterns["standard"]
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

func (s *Simulator) calculateOrderComplexity(items []string) float64 {
	if len(items) == 0 {
		return 1.0
	}

	totalComplexity := 0.0
	for _, itemID := range items {
		if item := s.getMenuItem(itemID); item != nil {
			totalComplexity += item.PrepComplexity
		}
	}

	// normalise complexity
	avgComplexity := totalComplexity / float64(len(items))

	// scale factor increases with number of items but not linearly
	scaleFactor := 1 + math.Log1p(float64(len(items)-1))*0.2

	return avgComplexity * scaleFactor
}

func (s *Simulator) calculateOrderMetrics(orders []models.Order) models.OrderMetrics {
	metrics := models.OrderMetrics{
		TotalOrders:  len(orders),
		PeakHours:    make(map[int]int),
		PopularItems: make(map[string]int),
	}

	var totalPrepTime, totalDeliveryTime float64
	var completedOrders, lateOrders, earlyOrders int

	for _, order := range orders {
		// revenue metrics
		metrics.TotalRevenue += order.TotalAmount

		// time metrics
		orderHour := order.OrderPlacedAt.Hour()
		metrics.PeakHours[orderHour]++

		// item popularity
		for _, itemID := range order.Items {
			metrics.PopularItems[itemID]++
		}

		// delivery performance
		if order.Status == models.OrderStatusDelivered {
			completedOrders++

			// calculate prep time
			if !order.PrepStartTime.IsZero() && !order.PickupTime.IsZero() {
				prepTime := order.PickupTime.Sub(order.PrepStartTime).Minutes()
				totalPrepTime += prepTime
			}

			// calculate delivery time
			if !order.PickupTime.IsZero() && !order.ActualDeliveryTime.IsZero() {
				deliveryTime := order.ActualDeliveryTime.Sub(order.PickupTime).Minutes()
				totalDeliveryTime += deliveryTime

				// check if delivery was on time
				if !order.EstimatedDeliveryTime.IsZero() {
					timeDiff := order.ActualDeliveryTime.Sub(order.EstimatedDeliveryTime).Minutes()
					if timeDiff > 10 {
						lateOrders++
					} else if timeDiff < -10 {
						earlyOrders++
					}
				}
			}
		}
	}

	// calculate averages
	if metrics.TotalOrders > 0 {
		metrics.AvgOrderValue = metrics.TotalRevenue / float64(metrics.TotalOrders)
	}
	if completedOrders > 0 {
		metrics.AvgPrepTime = totalPrepTime / float64(completedOrders)
		metrics.AvgDeliveryTime = totalDeliveryTime / float64(completedOrders)
		metrics.CompletionRate = float64(completedOrders) / float64(metrics.TotalOrders)
	}
	metrics.LateDeliveries = lateOrders
	metrics.EarlyDeliveries = earlyOrders

	return metrics
}

func (s *Simulator) generateBasePreparationTime(timeRange TimeRange) float64 {
	// generate preparation time using normal distribution
	prepTime := s.generateNormalizedRating(
		(timeRange.Min+timeRange.Max)/2,
		timeRange.Std,
		timeRange.Min,
		timeRange.Max,
	)
	return prepTime
}

func (s *Simulator) getTimeBasedPrepMultiplier(currentTime time.Time, restaurant *models.Restaurant) float64 {
	pattern := s.getRestaurantPattern(restaurant)
	hour := currentTime.Hour()
	weekday := currentTime.Weekday()

	// check if current hour is a peak hour
	isPeakHour := false
	if peakHours, exists := pattern.PeakHours[weekday]; exists {
		for _, peakHour := range peakHours {
			if hour == peakHour {
				isPeakHour = true
				break
			}
		}
	}

	if isPeakHour {
		return 1.2 // 20% slower during peak hours
	}

	// late night hours
	if hour >= 22 || hour <= 5 {
		return 1.1 // 10% slower during late night
	}

	return 1.0
}

func (s *Simulator) estimatePrepTime(restaurant *models.Restaurant, items []string) float64 {
	cluster := s.getRestaurantCluster(restaurant)
	pattern := s.getRestaurantPattern(restaurant)

	// base prep time from pattern
	baseTime := s.generateBasePreparationTime(pattern.PrepTimeRange)

	// adjust for restaurant cluster
	baseTime *= cluster.PreparationSpeed

	// adjust for current load
	currentLoad := float64(len(restaurant.CurrentOrders)) / float64(restaurant.Capacity)
	loadFactor := 1 + (currentLoad * 0.5) // Up to 50% slower when at capacity

	// adjust for time of day
	timeMultiplier := s.getTimeBasedPrepMultiplier(s.CurrentTime, restaurant)

	// adjust for order complexity
	complexityMultiplier := s.calculateOrderComplexity(items)

	finalPrepTime := baseTime * loadFactor * timeMultiplier * complexityMultiplier

	// add some randomness (±10%)
	randomFactor := 0.9 + (s.safeFloat64() * 0.2)

	return math.Max(restaurant.MinPrepTime, finalPrepTime*randomFactor)
}

func (s *Simulator) isUrbanArea(loc models.Location) bool {
	// Implement logic to determine if a location is in an urban area
	// This could be based on population density data or predefined urban zones
	// For simplicity, let's assume a central urban area:
	cityCenter := models.Location{Lat: s.Config.CityLat, Lon: s.Config.CityLon}
	return s.calculateDistance(loc, cityCenter) <= s.Config.UrbanRadius
}

func (s *Simulator) getPartnerIndex(partnerID string) int {
	for i := range s.DeliveryPartners {
		if i < len(s.DeliveryPartners) && s.DeliveryPartners[i].ID == partnerID {
			return i
		}
	}
	return -1
}

func (s *Simulator) moveTowardsHotspot(partner *models.DeliveryPartner, duration time.Duration) models.Location {
	cityCenter := models.Location{Lat: s.Config.CityLat, Lon: s.Config.CityLon}
	if s.calculateDistance(partner.CurrentLocation, cityCenter) > s.Config.NearLocationThreshold {
		// if partner is too far from city center, move towards it
		return s.moveTowards(partner.CurrentLocation, cityCenter, duration)
	}

	// find the nearest restaurant or hotspot
	nearestLocation := s.findNearestRestaurantOrHotspot(partner.CurrentLocation)

	// move towards the location
	return s.moveTowards(partner.CurrentLocation, nearestLocation, duration)
}

func (s *Simulator) findNearestHotspot(loc models.Location) models.Location {
	// define a list of hotspots
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

		// adjust distance by hotspot weight (more important hotspots seem "closer")
		adjustedDistance := distance / hotspot.Weight

		if adjustedDistance < minDistance {
			minDistance = adjustedDistance
			nearestHotspot = hotspot
		}
	}

	// add some randomness to the chosen hotspot
	jitter := 0.001 // About 111 meters
	nearestHotspot.Location.Lat += (s.safeFloat64() - 0.5) * jitter
	nearestHotspot.Location.Lon += (s.safeFloat64() - 0.5) * jitter

	return nearestHotspot.Location
}

func (s *Simulator) findNearestRestaurantOrHotspot(loc models.Location) models.Location {
	nearestDist := math.Inf(1)
	var nearestLoc models.Location

	// check nearby restaurants
	for _, restaurant := range s.Restaurants {
		dist := s.calculateDistance(loc, restaurant.Location)
		if dist < nearestDist {
			nearestDist = dist
			nearestLoc = restaurant.Location
		}
	}

	// if no nearby restaurant, use a hotspot
	if nearestDist == math.Inf(1) {
		nearestLoc = s.findNearestHotspot(loc)
	}

	return nearestLoc
}

func (s *Simulator) moveTowards(from, to models.Location, duration time.Duration) models.Location {
	distance := s.calculateDistance(from, to)
	speed := s.Config.PartnerMoveSpeed * (1 + (s.safeFloat64()*0.2 - 0.1)) // Add 10% randomness

	// calculate max distance that can be moved in this duration
	maxDistance := speed * duration.Hours()

	if distance <= maxDistance {
		// if we can reach the destination, go close to it but not exactly there
		//ratio := 0.99 // go 99% of the way there
		//return s.intermediatePoint(from, to, ratio)
		return to
	}

	// calculate the ratio of how far we can move
	ratio := maxDistance / distance

	//return s.intermediatePoint(from, to, ratio)
	return models.Location{
		Lat: from.Lat + (to.Lat-from.Lat)*ratio,
		Lon: from.Lon + (to.Lon-from.Lon)*ratio,
	}
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

func (s *Simulator) isAtLocation(loc1, loc2 models.Location) bool {
	distance := s.calculateDistance(loc1, loc2)
	return distance <= deliveryThreshold // consider locations the same if they're within 100 meters
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

	// adjust prep time based on current load
	adjustedPrepTime := restaurant.AvgPrepTime * loadFactor

	// ensure prep time doesn't go below minimum or become NaN
	if math.IsNaN(adjustedPrepTime) || adjustedPrepTime < restaurant.MinPrepTime {
		return restaurant.MinPrepTime
	}

	return adjustedPrepTime
}

func (s *Simulator) adjustPickupEfficiency(restaurant *models.Restaurant) float64 {
	recentOrders := s.getRecentCompletedOrdersByCount(restaurant.ID, 20) // consider last 20 orders
	if len(recentOrders) == 0 {
		return restaurant.PickupEfficiency // no recent orders, no change
	}

	var totalEfficiency float64
	for _, order := range recentOrders {
		actualPrepTime := order.PickupTime.Sub(order.PrepStartTime).Minutes()
		efficiency := restaurant.AvgPrepTime / actualPrepTime
		totalEfficiency += efficiency
	}
	avgEfficiency := totalEfficiency / float64(len(recentOrders))

	// gradually adjust towards new efficiency
	return restaurant.PickupEfficiency + (avgEfficiency-restaurant.PickupEfficiency)*s.Config.EfficiencyAdjustRate
}

func (s *Simulator) getOrderByID(orderID string) *models.Order {
	for i, order := range s.Orders {
		if order.ID == orderID {
			return &s.Orders[i]
		}
	}
	return nil
}

func (s *Simulator) getRecentOrders(userID string, count int) []models.Order {
	var recentOrders []models.Order

	// iterate through orders in reverse (assuming orders are stored chronologically)
	for i := len(s.Orders) - 1; i >= 0 && len(recentOrders) < count; i-- {
		if s.Orders[i].CustomerID == userID {
			recentOrders = append(recentOrders, s.Orders[i])
		}
	}

	return recentOrders
}

func (s *Simulator) getRecentCompletedOrdersByCount(restaurantID string, count int) []models.Order {
	var recentCompletedOrders []models.Order

	// iterate through orders in reverse (assuming orders are stored chronologically)
	for i := len(s.Orders) - 1; i >= 0 && len(recentCompletedOrders) < count; i-- {
		if s.Orders[i].RestaurantID == restaurantID && s.Orders[i].Status == models.OrderStatusDelivered {
			recentCompletedOrders = append(recentCompletedOrders, s.Orders[i])
		}
	}

	return recentCompletedOrders
}

func (s *Simulator) getRecentCompletedOrders(restaurantID string, duration time.Duration) []models.Order {
	// calculate cutoff time
	cutoffTime := s.CurrentTime.Add(-duration)

	// create map to track unique orders
	uniqueOrders := make(map[string]models.Order)

	// first check CompletedOrdersByRestaurant cache
	if cachedOrders, exists := s.CompletedOrdersByRestaurant[restaurantID]; exists {
		for _, order := range cachedOrders {
			if order.OrderPlacedAt.After(cutoffTime) &&
				order.Status == models.OrderStatusDelivered {
				uniqueOrders[order.ID] = order
			}
		}
	}

	// then scan through all orders to ensure completeness
	for _, order := range s.Orders {
		if order.RestaurantID == restaurantID &&
			order.OrderPlacedAt.After(cutoffTime) &&
			order.Status == models.OrderStatusDelivered {
			uniqueOrders[order.ID] = order
		}
	}

	// convert map to slice
	recentOrders := make([]models.Order, 0, len(uniqueOrders))
	for _, order := range uniqueOrders {
		recentOrders = append(recentOrders, order)
	}

	// sort by order time (most recent first)
	sort.Slice(recentOrders, func(i, j int) bool {
		return recentOrders[i].OrderPlacedAt.After(recentOrders[j].OrderPlacedAt)
	})

	// calculate and track some metrics for the restaurant
	if len(recentOrders) > 0 {
		metrics := s.calculateOrderMetrics(recentOrders)
		s.updateRestaurantOrderMetrics(restaurantID, metrics)
	}

	return recentOrders
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
	// initialise traffic conditions for different times of the day
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

func (s *Simulator) intermediatePoint(from, to models.Location, fraction float64) models.Location {
	// convert to radians
	φ1 := from.Lat * math.Pi / 180
	λ1 := from.Lon * math.Pi / 180
	φ2 := to.Lat * math.Pi / 180
	λ2 := to.Lon * math.Pi / 180

	// calculate intermediate point
	a := math.Sin((1-fraction)*φ1) * math.Cos(fraction*φ2)
	b := math.Sin((1-fraction)*φ2) * math.Cos(fraction*φ1)
	x := a + b
	y := math.Cos((1-fraction)*φ1) * math.Cos(fraction*φ2) * math.Cos(λ2-λ1)
	φ3 := math.Atan2(x, y)
	λ3 := λ1 + math.Atan2(math.Sin(λ2-λ1)*math.Cos(fraction*φ2),
		math.Cos((1-fraction)*φ1)*math.Cos(fraction*φ2)-
			math.Sin((1-fraction)*φ1)*math.Sin(fraction*φ2)*math.Cos(λ2-λ1))

	// convert back to degrees
	return models.Location{
		Lat: φ3 * 180 / math.Pi,
		Lon: λ3 * 180 / math.Pi,
	}
}

// Time and pattern-related helpers
func (s *Simulator) calculateTimeBasedOrderProbability(user *models.User, currentTime time.Time) float64 {
	segment := models.DefaultCustomerSegments[user.Segment]
	hour := currentTime.Hour()
	weekday := currentTime.Weekday()

	// base probability from segment's orders per month
	baseProbability := segment.OrdersPerMonth / (30 * 24.0)

	// time-based multipliers
	multiplier := 1.0

	// weekend multiplier
	if weekday == time.Saturday || weekday == time.Sunday {
		multiplier *= 1.5
	}

	// Friday night multiplier (increases as the night goes on)
	if weekday == time.Friday && hour >= 17 {
		nightProgress := float64(hour-17) / 7.0   // 17:00 to 24:00
		multiplier *= 1.3 + (nightProgress * 0.5) // Up to 1.8x at peak
	}

	// peak hours multiplier based on segment
	if isWeekdayPeakHour(hour) {
		multiplier *= segment.PeakHourBias
	}

	// adjust for user's historical patterns
	if patterns, exists := user.PurchasePatterns[weekday]; exists {
		for _, pastHour := range patterns {
			if pastHour == hour {
				multiplier *= 1.2 // Boost for preferred ordering times
				break
			}
		}
	}

	// weather factor (could be expanded)
	if s.isRainyOrColdWeather() {
		multiplier *= 1.3 // people order more in bad weather
	}

	return baseProbability * multiplier
}

func (s *Simulator) getSpecialEventMultiplier() float64 {
	// Check for special dates/events
	specialDates := map[string]float64{
		"12-31": 2.0, // New Year's Eve
		"02-14": 1.8, // Valentine's Day
		"12-25": 0.5, // Christmas Day
		// Add more special dates
	}

	dateKey := s.CurrentTime.Format("01-02")
	if multiplier, exists := specialDates[dateKey]; exists {
		return multiplier
	}

	// Check for sporting events, local festivals, etc.
	// This could be expanded based on your needs
	return 1.0
}

func (s *Simulator) calculateBaseDeliveryRating(timeDifference time.Duration) float64 {
	minutesDifference := timeDifference.Minutes()

	switch {
	case minutesDifference <= -10:
		return 5.0
	case minutesDifference <= 0:
		return 4.5
	case minutesDifference <= 10:
		return 4.0
	case minutesDifference <= 20:
		return 3.0
	case minutesDifference <= 30:
		return 2.0
	default:
		return 1.0
	}
}

func (s *Simulator) getTimeBasedRatingWeight(orderTime time.Time) float64 {
	hour := orderTime.Hour()

	if s.isPeakHour(orderTime) {
		return RatingDistributions.TimeWeights["peak_hours"]
	} else if hour >= 23 || hour <= 5 {
		return RatingDistributions.TimeWeights["off_peak"]
	}
	return RatingDistributions.TimeWeights["normal_hours"]
}

func (s *Simulator) calculateBaseTemperature() float64 {
	// base temperature follows a sinusoidal pattern through the year
	// assuming config.CityLat determines hemisphere

	// day of year from 0 to 365
	dayOfYear := float64(s.CurrentTime.YearDay())

	// phase shift based on hemisphere (6 months = π)
	phaseShift := 0.0
	if s.Config.CityLat < 0 {
		// southern hemisphere
		phaseShift = math.Pi
	}

	// calculate where we are in the annual temperature cycle
	// temperature follows a sine wave with period of 1 year
	annualCycle := math.Sin((2 * math.Pi * dayOfYear / 365.0) + phaseShift)

	// base temperature range depends on latitude
	latitude := math.Abs(s.Config.CityLat)

	var tempRange, avgTemp float64

	switch {
	case latitude < 23.5: // tropical
		tempRange = 10 // ±5°C variation
		avgTemp = 25
	case latitude < 45: // Temperate
		tempRange = 20 // ±10°C variation
		avgTemp = 15
	default: // polar/subpolar
		tempRange = 30 // ±15°C variation
		avgTemp = 5
	}

	return avgTemp + (tempRange/2)*annualCycle
}

func (s *Simulator) getTimeOfDayTempAdjustment() float64 {
	hour := float64(s.CurrentTime.Hour())

	// temperature follows a sine wave with period of 24 hours
	// minimum at around 5 AM (5 hours = ~π/2.4 radians in 24-hour cycle)
	hourlyPhase := (2 * math.Pi * (hour - 5)) / 24.0

	// daily temperature variation depends on climate and season
	month := s.CurrentTime.Month()
	latitude := math.Abs(s.Config.CityLat)

	var dailyRange float64

	// larger temperature swings in:
	// - higher latitudes (not tropics)
	// - dry climates
	// - summer months
	switch {
	case latitude < 23.5: // Tropical
		dailyRange = 8 // ±4°C variation
	case latitude < 45: // Temperate
		if month >= time.June && month <= time.August {
			dailyRange = 15 // ±7.5°C variation in summer
		} else {
			dailyRange = 10 // ±5°C variation in winter
		}
	default: // Polar/Subpolar
		if month >= time.June && month <= time.August {
			dailyRange = 20 // ±10°C variation in summer
		} else {
			dailyRange = 8 // ±4°C variation in winter
		}
	}

	return (dailyRange / 2) * math.Sin(hourlyPhase)
}

func (s *Simulator) getRandomTempVariation() float64 {
	// random variation that persists for some time
	// use the hour as a seed for consistency within the hour
	hourKey := s.CurrentTime.Format("2006010215")

	// use a hash of the hour to generate consistent random variation
	hash := fnv.New32()
	_, err := hash.Write([]byte(hourKey))
	if err != nil {
		return 0
	}
	hashValue := float64(hash.Sum32()) / float64(math.MaxUint32)

	// convert to temperature variation (-2 to +2 degrees)
	return (hashValue * 4) - 2
}

func (s *Simulator) getCurrentTemperature() float64 {
	baseTemp := s.calculateBaseTemperature()
	timeAdjustment := s.getTimeOfDayTempAdjustment()
	randomVariation := s.getRandomTempVariation()

	var weatherEffect float64
	if s.WeatherState != nil {
		switch s.WeatherState.Condition {
		case "rain", "cloudy", "overcast":
			weatherEffect = -2
		case "snow":
			weatherEffect = -8
		case "storm":
			weatherEffect = -4
		case "clear":
			weatherEffect = 1
		}
	}

	finalTemp := baseTemp + timeAdjustment + randomVariation + weatherEffect

	// cap temperature within realistic bounds for the location
	return math.Max(-20.0, math.Min(40.0, finalTemp))
}

func (s *Simulator) getCurrentWeather() string {
	// check if we need to generate new weather
	if s.WeatherState == nil || s.CurrentTime.After(s.WeatherState.StartTime.Add(s.WeatherState.Duration)) {
		s.generateNewWeather()
	}

	return s.WeatherState.Condition
}

func (s *Simulator) getWeatherTempEffect() float64 {
	if s.WeatherState == nil {
		return 0
	}

	weather := s.WeatherState.Condition

	// temperature adjustments based on weather conditions
	weatherEffects := map[string]struct {
		min float64
		max float64
	}{
		"clear":    {0, 2},    // clear skies can be slightly warmer
		"cloudy":   {-1, 0},   // clouds moderate temperature
		"rain":     {-3, -1},  // rain usually means cooler
		"snow":     {-10, -5}, // snow requires cold temperatures
		"storm":    {-5, -2},  // storms usually bring temperature drops
		"fog":      {-2, 0},   // fog often occurs in cooler conditions
		"overcast": {-2, -1},  // overcast skies reduce temperature
	}

	if effect, exists := weatherEffects[weather]; exists {
		// get a consistent random value for the current hour
		hourKey := s.CurrentTime.Format("2006010215")
		hash := fnv.New32()
		_, err := hash.Write([]byte(hourKey))
		if err != nil {
			return 0
		}
		randomValue := float64(hash.Sum32()) / float64(math.MaxUint32)

		// interpolate between min and max effect
		return effect.min + (effect.max-effect.min)*randomValue
	}

	return 0
}

func (s *Simulator) generateNewWeather() {
	if s.WeatherState == nil {
		// initialise with clear weather if no current state
		s.WeatherState = &WeatherState{
			Condition: "clear",
			StartTime: s.CurrentTime,
			Duration:  4 * time.Hour,
		}
		return
	}

	currentSeason := s.getCurrentSeason()
	baseTemp := s.calculateBaseTemperature()

	// get possible transitions from current weather
	possibleTransitions := weatherTransitions[s.WeatherState.Condition]

	// calculate transition probabilities
	var totalProbability float64
	adjustedProbabilities := make([]float64, len(possibleTransitions))

	for i, transition := range possibleTransitions {
		probability := transition.BaseProbability

		// apply seasonal modifier
		if modifier, exists := seasonalModifiers[currentSeason][transition.Condition]; exists {
			probability *= modifier
		}

		// Simplify temperature checks
		if transition.Condition == "snow" && baseTemp > 2 {
			probability = 0 // No snow above 2°C
		}

		// apply time of day modifier
		probability *= s.getTimeOfDayWeatherModifier(transition.Condition)

		// apply geographic modifier
		probability *= s.getGeographicWeatherModifier(transition.Condition)

		adjustedProbabilities[i] = probability
		totalProbability += probability
	}

	// normalize probabilities
	for i := range adjustedProbabilities {
		adjustedProbabilities[i] /= totalProbability
	}

	// select new weather condition
	randVal := s.safeFloat64()
	cumulativeProbability := 0.0

	var selectedTransition WeatherTransition
	for i, probability := range adjustedProbabilities {
		cumulativeProbability += probability
		if randVal <= cumulativeProbability {
			selectedTransition = possibleTransitions[i]
			break
		}
	}

	// if no transition was selected, keep current weather
	if selectedTransition.Condition == "" {
		selectedTransition = WeatherTransition{
			Condition:   s.WeatherState.Condition,
			MinDuration: 1 * time.Hour,
			MaxDuration: 4 * time.Hour,
		}
	}

	// generate duration for new weather
	duration := s.generateWeatherDuration(selectedTransition)

	// create new weather state
	s.WeatherState = &WeatherState{
		Condition:     selectedTransition.Condition,
		StartTime:     s.CurrentTime,
		Duration:      duration,
		Intensity:     s.generateWeatherIntensity(selectedTransition.Condition),
		WindSpeed:     s.generateWindSpeed(selectedTransition.Condition),
		Humidity:      s.generateHumidity(selectedTransition.Condition),
		Precipitation: s.generatePrecipitation(selectedTransition.Condition),
	}
}

func (s *Simulator) getCurrentSeason() string {
	month := s.CurrentTime.Month()
	isNorthernHemisphere := s.Config.CityLat >= 0

	switch {
	case month >= time.March && month <= time.May:
		return "spring"
	case month >= time.June && month <= time.August:
		if isNorthernHemisphere {
			return "summer"
		}
		return "winter"
	case month >= time.September && month <= time.November:
		return "fall"
	default:
		if isNorthernHemisphere {
			return "winter"
		}
		return "summer"
	}
}

func (s *Simulator) getTemperatureWeatherModifier(temp float64, condition string) float64 {
	switch condition {
	case "snow":
		if temp > 2 {
			return 0.0 // no snow above 2°C
		}
		return 1.0 - (temp+5.0)/7.0 // more likely the colder it gets below 2°C
	case "rain":
		if temp < 0 {
			return 0.0 // no rain below freezing
		}
		return 1.0
	case "storm":
		if temp > 25 {
			return 1.5 // more storms in hot weather
		}
		return 1.0
	default:
		return 1.0
	}
}

func (s *Simulator) getTimeOfDayWeatherModifier(condition string) float64 {
	hour := s.CurrentTime.Hour()

	switch condition {
	case "fog":
		// more likely in early morning
		if hour >= 4 && hour <= 9 {
			return 2.0
		}
		return 0.5
	case "storm":
		// more likely in afternoon/evening
		if hour >= 14 && hour <= 20 {
			return 1.5
		}
		return 1.0
	case "clear":
		// more likely during midday
		if hour >= 10 && hour <= 16 {
			return 1.3
		}
		return 1.0
	default:
		return 1.0
	}
}

func (s *Simulator) getGeographicWeatherModifier(condition string) float64 {
	// modify weather probabilities based on geographic location
	latitude := math.Abs(s.Config.CityLat)

	switch {
	case latitude < 23.5: // Tropical
		switch condition {
		case "rain":
			return 1.5 // More rain in tropics
		case "snow":
			return 0.0 // No snow in tropics
		case "clear":
			return 1.2
		}
	case latitude > 45: // Polar/Subpolar
		switch condition {
		case "snow":
			return 1.5 // More snow in polar regions
		case "rain":
			return 0.8
		}
	}

	return 1.0
}

func (s *Simulator) generateWeatherDuration(transition WeatherTransition) time.Duration {
	minDuration := transition.MinDuration
	maxDuration := transition.MaxDuration

	// generate random duration between min and max
	durationRange := maxDuration - minDuration
	randomDuration := time.Duration(s.safeFloat64() * float64(durationRange))

	return minDuration + randomDuration
}

func (s *Simulator) generateWeatherIntensity(condition string) float64 {
	// generate base intensity
	intensity := 0.3 + (s.safeFloat64() * 0.7)

	// adjust based on condition
	switch condition {
	case "storm":
		intensity = math.Max(0.7, intensity) // Storms are always intense
	case "clear", "cloudy":
		intensity = math.Min(0.8, intensity) // Clear/cloudy usually less intense
	}

	return intensity
}

func (s *Simulator) generateWindSpeed(condition string) float64 {
	baseSpeed := 5.0 + (s.safeFloat64() * 15.0) // 5-20 km/h base

	switch condition {
	case "storm":
		return baseSpeed * 2.5 // much stronger winds in storms
	case "clear":
		return baseSpeed * 0.7 // generally calmer
	case "snow":
		return baseSpeed * 1.3 // often windy with snow
	default:
		return baseSpeed
	}
}

func (s *Simulator) generateHumidity(condition string) float64 {
	baseHumidity := 0.4 + (s.safeFloat64() * 0.3) // 40-70% base

	switch condition {
	case "rain", "storm":
		return math.Min(1.0, baseHumidity*1.4) // higher humidity
	case "snow":
		return baseHumidity * 0.9 // lower humidity
	case "clear":
		return baseHumidity * 0.8 // lower humidity
	default:
		return baseHumidity
	}
}

func (s *Simulator) generatePrecipitation(condition string) float64 {
	switch condition {
	case "rain":
		return 1.0 + (s.safeFloat64() * 9.0) // 1-10mm/hour
	case "storm":
		return 10.0 + (s.safeFloat64() * 20.0) // 10-30mm/hour
	case "snow":
		return 1.0 + (s.safeFloat64() * 4.0) // 1-5mm/hour equivalent
	default:
		return 0.0
	}
}

func (s *Simulator) canSnow() bool {
	return s.getCurrentTemperature() <= 2.0 // snow possible below 2°C
}

func (s *Simulator) isExtremeTemperature() bool {
	temp := s.getCurrentTemperature()
	return temp <= -10 || temp >= 35
}

func (s *Simulator) getTemperatureComfortLevel() string {
	temp := s.getCurrentTemperature()
	switch {
	case temp < 0:
		return "freezing"
	case temp < 10:
		return "cold"
	case temp < 20:
		return "cool"
	case temp < 25:
		return "pleasant"
	case temp < 30:
		return "warm"
	case temp < 35:
		return "hot"
	default:
		return "extreme_heat"
	}
}

func (s *Simulator) getWeatherRatingWeight() float64 {
	if s.isRainyOrColdWeather() {
		return RatingDistributions.WeatherEffects["rain"]
	}
	return RatingDistributions.WeatherEffects["normal"]
}

func (s *Simulator) isRainyOrColdWeather() bool {
	// Simple weather simulation based on time of year
	month := s.CurrentTime.Month()

	// Higher chance of bad weather in winter months
	baseChance := 0.2
	if month >= time.November && month <= time.February {
		baseChance = 0.4
	} else if month >= time.March && month <= time.April {
		baseChance = 0.3
	}

	return s.safeFloat64() < baseChance
}

func (s *Simulator) getTimeBasedCapacityMultiplier(currentTime time.Time) float64 {
	hour := currentTime.Hour()
	weekday := currentTime.Weekday()

	// base multiplier
	multiplier := 1.0

	// time-based adjustments
	switch {
	case hour >= 11 && hour <= 14: // lunch hour rush
		multiplier *= 1.3
		if weekday >= time.Monday && weekday <= time.Friday {
			multiplier *= 1.2 // extra busy during weekday lunches
		}

	case hour >= 18 && hour <= 21: // dinner rush
		multiplier *= 1.4
		if weekday == time.Friday || weekday == time.Saturday {
			multiplier *= 1.3 // busier weekend dinners
		}

	case hour >= 14 && hour <= 17: // afternoon lull
		multiplier *= 0.7

	case hour >= 22 || hour <= 5: // late night/early morning
		multiplier *= 0.5
		if weekday == time.Friday || weekday == time.Saturday {
			multiplier *= 1.5 // late night weekend boost
		}

	case hour >= 6 && hour <= 10: // breakfast
		multiplier *= 0.8
		if weekday >= time.Monday && weekday <= time.Friday {
			multiplier *= 1.2 // busier weekday breakfasts
		}
	}

	// day-specific adjustments
	switch weekday {
	case time.Friday:
		multiplier *= 1.2 // generally busier on Fridays
	case time.Saturday:
		multiplier *= 1.3 // busiest day
	case time.Sunday:
		// different patterns for Sunday
		if hour >= 10 && hour <= 15 { // Sunday brunch
			multiplier *= 1.4
		} else {
			multiplier *= 0.9
		}
	case time.Monday:
		multiplier *= 0.9 // usually slower
	}

	// monthly patterns
	month := currentTime.Month()
	switch month {
	case time.December: // holiday season
		multiplier *= 1.3
	case time.January: // post-holiday slowdown
		multiplier *= 0.8
	case time.July, time.August: // summer months
		if hour >= 18 { // busier summer evenings
			multiplier *= 1.2
		}
	}

	// pay period effect (assuming bi-weekly pay periods)
	dayOfMonth := currentTime.Day()
	if dayOfMonth == 15 || dayOfMonth <= 2 {
		multiplier *= 1.2 // busier around paydays
	}

	return math.Max(0.5, math.Min(2.0, multiplier)) // Cap between 50% and 200%
}

func (s *Simulator) getWeatherCapacityMultiplier() float64 {
	weather := s.getCurrentWeather()
	baseMultiplier := 1.0

	// weather condition adjustments
	weatherMultipliers := map[string]struct {
		capacity float64
		delivery float64
	}{
		"rain": {
			capacity: 1.3, // more orders during rain
			delivery: 0.8, // slower delivery times
		},
		"snow": {
			capacity: 1.4, // even more orders during snow
			delivery: 0.6, // much slower delivery times
		},
		"extreme_heat": {
			capacity: 1.2, // more orders during extreme heat
			delivery: 0.9, // slightly slower delivery
		},
		"pleasant": {
			capacity: 0.9, // fewer orders in nice weather
			delivery: 1.1, // faster delivery times
		},
		"storm": {
			capacity: 1.5, // highest order volume
			delivery: 0.5, // slowest delivery times
		},
	}

	if multipliers, exists := weatherMultipliers[weather]; exists {
		baseMultiplier = multipliers.capacity
	}

	// temperature impact
	temperature := s.getCurrentTemperature()
	switch {
	case temperature < 0:
		baseMultiplier *= 1.3 // cold weather increases orders
	case temperature > 30:
		baseMultiplier *= 1.2 // hot weather increases orders
	case temperature >= 15 && temperature <= 25:
		baseMultiplier *= 0.9 // pleasant weather decreases orders
	}

	// time of day weather impact
	hour := s.CurrentTime.Hour()
	if hour >= 17 && hour <= 22 { // evening hours
		switch weather {
		case "rain", "snow", "storm":
			baseMultiplier *= 1.2 // even more impact during dinner hours
		}
	}

	return math.Max(0.5, math.Min(2.0, baseMultiplier))
}

func (s *Simulator) getSpecialEventCapacityMultiplier() float64 {
	multiplier := 1.0
	currentDate := s.CurrentTime

	// check for special dates
	specialDates := map[string]float64{
		"12-31": 1.8, // New Year's Eve
		"12-24": 1.5, // Christmas Eve
		"12-25": 0.3, // Christmas Day (most places closed)
		"01-01": 1.7, // New Year's Day
		"02-14": 1.6, // Valentine's Day
		"10-31": 1.4, // Halloween
	}

	dateKey := currentDate.Format("01-02")
	if multiplier, exists := specialDates[dateKey]; exists {
		return multiplier
	}

	// local events impact
	localEvents := s.getLocalEvents(currentDate)
	for _, event := range localEvents {
		switch event.Type {
		case "sports_game":
			multiplier *= 1.3
		case "concert":
			multiplier *= 1.2
		case "festival":
			// festivals might actually reduce delivery orders
			multiplier *= 0.8
		case "convention":
			multiplier *= 1.4
		}
	}

	// academic calendar effects
	if s.isUniversityArea() {
		switch {
		case s.isFinalsWeek():
			multiplier *= 1.5
		case s.isMovingWeek():
			multiplier *= 1.3
		case s.isSpringBreak():
			multiplier *= 0.7
		}
	}

	// add randomness for unexpected events
	if s.safeFloat64() < 0.05 { // 5% chance of random event
		randomFactor := 0.8 + (s.safeFloat64() * 0.4) // random factor between 0.8 and 1.2
		multiplier *= randomFactor
	}

	return math.Max(0.3, math.Min(2.0, multiplier)) // cap between 30% and 200%
}

func (s *Simulator) getLocalEvents(date time.Time) []LocalEvent {
	// in a real system, this would query a database of local events
	// for this simulation, we'll generate some random events
	events := make([]LocalEvent, 0)

	s.rngMutex.Lock()
	shouldGenerateEvent := s.safeFloat64() < 0.2 // 20% chance of a local event
	eventIndex := s.safeIntn(4)                  // For selecting event type
	s.rngMutex.Unlock()

	if shouldGenerateEvent {
		eventTypes := []string{"sports_game", "concert", "festival", "convention"}
		eventType := eventTypes[eventIndex]

		event := LocalEvent{
			Type:      eventType,
			StartTime: date,
			EndTime:   date.Add(4 * time.Hour),
		}
		events = append(events, event)
	}

	return events
}

func (s *Simulator) isUniversityArea() bool {
	// this would check if the restaurant is near a university
	// for simulation, return true 20% of the time
	return s.safeFloat64() < 0.2
}

func (s *Simulator) isFinalsWeek() bool {
	// rough approximation of finals weeks
	month := s.CurrentTime.Month()
	day := s.CurrentTime.Day()
	return (month == time.December && day >= 10 && day <= 20) ||
		(month == time.May && day >= 1 && day <= 10)
}

func (s *Simulator) isMovingWeek() bool {
	// typical university moving weeks
	month := s.CurrentTime.Month()
	day := s.CurrentTime.Day()
	return (month == time.August && day >= 15 && day <= 31) ||
		(month == time.May && day >= 1 && day <= 15)
}

func (s *Simulator) isSpringBreak() bool {
	// typical spring break period
	month := s.CurrentTime.Month()
	day := s.CurrentTime.Day()
	return month == time.March && day >= 10 && day <= 20
}

func (s *Simulator) isHoliday() bool {
	// format date as MM-DD
	dateKey := s.CurrentTime.Format("01-02")

	holidays := map[string]bool{
		"12-24": true, // Christmas Eve
		"12-25": true, // Christmas
		"12-31": true, // New Year's Eve
		"01-01": true, // New Year's Day
	}

	return holidays[dateKey]
}

func (s *Simulator) calculateSeasonalGrowthRate(baseRate float64) float64 {
	month := s.CurrentTime.Month()

	// seasonal adjustments
	seasonalFactors := map[time.Month]float64{
		time.January:   0.8, // Post-holiday slowdown
		time.February:  0.9,
		time.March:     1.0,
		time.April:     1.1,
		time.May:       1.2,
		time.June:      1.3, // Summer growth
		time.July:      1.3,
		time.August:    1.2,
		time.September: 1.1, // Back to school/work
		time.October:   1.0,
		time.November:  1.1, // Holiday season starting
		time.December:  1.2, // Peak holiday season
	}

	seasonalFactor := seasonalFactors[month]

	// apply week of month adjustment
	dayOfMonth := s.CurrentTime.Day()
	weekOfMonth := (dayOfMonth - 1) / 7
	weekFactors := []float64{1.2, 1.0, 0.9, 0.8, 0.9} // weekly pattern
	weekFactor := weekFactors[weekOfMonth]

	return baseRate * seasonalFactor * weekFactor
}

func (s *Simulator) safeFloat64() float64 {
	//s.rngMutex.Lock()
	//defer s.rngMutex.Unlock()
	return s.Rng.Float64()
}

func (s *Simulator) safeIntn(n int) int {
	if n <= 0 {
		return 0
	}
	//s.rngMutex.Lock()
	//defer s.rngMutex.Unlock()
	return s.Rng.Intn(n)
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

func degreesToRadians(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

func getDayPart(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

func containsAny(s string, keywords []string) bool {
	s = strings.ToLower(s)
	for _, keyword := range keywords {
		if strings.Contains(s, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func updateRating(currentRating, newRating, alpha float64) float64 {
	updatedRating := (alpha * newRating) + ((1 - alpha) * currentRating)
	return math.Max(1, math.Min(5, updatedRating))
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
