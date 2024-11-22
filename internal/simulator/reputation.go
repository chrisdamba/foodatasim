package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	RatingWindows = []RatingWindow{
		{Duration: 7 * 24 * time.Hour, Weight: 0.4, MinReviews: 5},    // Last week
		{Duration: 30 * 24 * time.Hour, Weight: 0.3, MinReviews: 15},  // Last month
		{Duration: 90 * 24 * time.Hour, Weight: 0.2, MinReviews: 30},  // Last quarter
		{Duration: 365 * 24 * time.Hour, Weight: 0.1, MinReviews: 50}, // Last year
	}

	ReputationThresholds = map[string]float64{
		"excellent": 4.5,
		"good":      4.0,
		"average":   3.5,
		"poor":      3.0,
	}

	PriceQualityThresholds = map[string]struct {
		RatingThreshold float64
		PriceMultiplier float64
	}{
		"premium":  {4.3, 1.5},
		"standard": {3.8, 1.0},
		"budget":   {3.5, 0.8},
	}
)

func (s *Simulator) updateRestaurantReputation(restaurant *models.Restaurant, review models.Review) {
	metrics := s.calculateReputationMetrics(restaurant)

	// update base rating with time-weighted approach
	newRating := s.calculateTimeWeightedRating(restaurant, review)

	// adjust for consistency
	consistencyFactor := metrics.ConsistencyScore
	newRating = (newRating * 0.8) + (newRating * 0.2 * consistencyFactor)

	// apply trend adjustment
	trendAdjustment := metrics.TrendScore * 0.1 // Max 10% impact
	newRating += trendAdjustment

	// reliability impact
	reliabilityImpact := (metrics.ReliabilityScore - 0.5) * 0.2 // ±10% impact
	newRating *= (1 + reliabilityImpact)

	// price-quality relationship
	priceQualityAdjustment := (metrics.PriceQualityScore - 1.0) * 0.15
	newRating *= (1 + priceQualityAdjustment)

	// ensure rating stays within bounds
	restaurant.Rating = math.Max(1.0, math.Min(5.0, newRating))
	restaurant.TotalRatings++

	// update reputation metrics
	s.updateReputationMetrics(restaurant, metrics, review)
}

func (s *Simulator) calculateReputationMetrics(restaurant *models.Restaurant) models.ReputationMetrics {
	metrics := models.ReputationMetrics{
		BaseRating: restaurant.Rating,
		LastUpdate: s.CurrentTime,
	}

	// calculate consistency score
	metrics.ConsistencyScore = s.calculateConsistencyScore(restaurant)

	// calculate trend score
	metrics.TrendScore = s.calculateTrendScore(restaurant)

	// calculate reliability score
	metrics.ReliabilityScore = s.calculateReliabilityScore(restaurant)

	// calculate price-quality score
	metrics.PriceQualityScore = s.calculatePriceQualityScore(restaurant)

	return metrics
}

func (s *Simulator) calculateTimeWeightedRating(restaurant *models.Restaurant, newReview models.Review) float64 {
	totalWeight := 0.0
	weightedSum := 0.0

	for _, window := range RatingWindows {
		reviews := s.getReviewsInWindow(restaurant.ID, window.Duration)
		if len(reviews) >= window.MinReviews {
			windowRating := s.calculateWindowRating(reviews)
			weightedSum += windowRating * window.Weight
			totalWeight += window.Weight
		}
	}

	// if no windows have enough reviews, use simple average
	if totalWeight == 0 {
		return (restaurant.Rating*float64(restaurant.TotalRatings) + newReview.OverallRating) /
			float64(restaurant.TotalRatings+1)
	}

	return weightedSum / totalWeight
}

func (s *Simulator) calculateConsistencyScore(restaurant *models.Restaurant) float64 {
	reviews := s.getRecentReviews(restaurant.ID, 30*24*time.Hour) // Last 30 days
	if len(reviews) < 10 {
		return 1.0 // not enough data
	}

	// calculate standard deviation
	var sum, sumSq float64
	for _, review := range reviews {
		sum += review.OverallRating
		sumSq += review.OverallRating * review.OverallRating
	}

	mean := sum / float64(len(reviews))
	variance := (sumSq / float64(len(reviews))) - (mean * mean)
	stdDev := math.Sqrt(variance)

	// convert to consistency score (lower std dev = higher consistency)
	consistencyScore := 1.0 - (stdDev / 5.0) // 5.0 is max possible std dev

	return math.Max(0.5, math.Min(1.5, consistencyScore))
}

func (s *Simulator) calculateTrendScore(restaurant *models.Restaurant) float64 {
	reviews := s.getRecentReviews(restaurant.ID, 90*24*time.Hour) // Last 90 days
	if len(reviews) < 20 {
		return 0.0 // not enough data
	}

	// calculate weekly averages
	weeklyRatings := make(map[int]float64)
	weeksCounts := make(map[int]int)

	for _, review := range reviews {
		week := int(s.CurrentTime.Sub(review.CreatedAt).Hours() / (24 * 7))
		weeklyRatings[week] += review.OverallRating
		weeksCounts[week]++
	}

	// calculate trend
	var x, y, xy, xx float64
	n := float64(len(weeklyRatings))

	for week, rating := range weeklyRatings {
		avgRating := rating / float64(weeksCounts[week])
		x += float64(week)
		y += avgRating
		xy += float64(week) * avgRating
		xx += float64(week) * float64(week)
	}

	// calculate trend slope
	slope := ((n * xy) - (x * y)) / ((n * xx) - (x * x))

	// normalize trend score to [-1, 1]
	return math.Max(-1, math.Min(1, slope*10))
}

func (s *Simulator) calculateReliabilityScore(restaurant *models.Restaurant) float64 {
	recentOrders := s.getRecentCompletedOrdersByCount(restaurant.ID, 20)
	if len(recentOrders) == 0 {
		return 1.0
	}

	var reliabilitySum float64
	for _, order := range recentOrders {
		// calculate preparation time accuracy
		estimatedPrep := order.EstimatedPickupTime.Sub(order.PrepStartTime)
		actualPrep := order.PickupTime.Sub(order.PrepStartTime)
		prepAccuracy := 1.0 - math.Min(1.0, math.Abs(actualPrep.Minutes()-estimatedPrep.Minutes())/30.0)

		// calculate order accuracy (if all items were available)
		orderAccuracy := 1.0 // could be adjusted based on item availability

		reliabilitySum += prepAccuracy*0.7 + orderAccuracy*0.3
	}

	return reliabilitySum / float64(len(recentOrders))
}

func (s *Simulator) calculatePriceQualityScore(restaurant *models.Restaurant) float64 {
	avgItemPrice := s.calculateAverageItemPrice(restaurant)
	recentRating := s.calculateRecentAverageRating(restaurant)

	// determine price tier
	var priceQualityScore float64
	for _, thresholds := range PriceQualityThresholds {
		if avgItemPrice >= thresholds.PriceMultiplier*20.0 { // Base price of 20.0
			if recentRating >= thresholds.RatingThreshold {
				priceQualityScore = 1.2 // exceeding expectations for price point
			} else {
				priceQualityScore = 0.8 // not meeting expectations for price point
			}
			break
		}
	}

	return priceQualityScore
}

func (s *Simulator) updateReputationMetrics(restaurant *models.Restaurant, metrics models.ReputationMetrics, review models.Review) {
	// store historical metrics for trend analysis
	if restaurant.ReputationHistory == nil {
		restaurant.ReputationHistory = make([]models.ReputationMetrics, 0)
	}

	metrics.LastUpdate = s.CurrentTime
	restaurant.ReputationHistory = append(restaurant.ReputationHistory, metrics)

	// keep only last 90 days of history
	for i := 0; i < len(restaurant.ReputationHistory); i++ {
		if s.CurrentTime.Sub(restaurant.ReputationHistory[i].LastUpdate) > 90*24*time.Hour {
			restaurant.ReputationHistory = restaurant.ReputationHistory[i+1:]
			i--
		}
	}

	// update restaurant status based on metrics
	s.updateRestaurantStatusBasedOnReputation(restaurant, metrics)
}

func (s *Simulator) calculateBasePopularity(restaurant *models.Restaurant) float64 {
	if restaurant == nil {
		return 0.0
	}

	// start with rating-based popularity
	basePopularity := (restaurant.Rating / 5.0) * 0.4 // 40% weight to rating

	// recent order volume (last 7 days)
	recentOrders := s.getRecentCompletedOrders(restaurant.ID, 7*24*time.Hour)
	orderVolumeFactor := float64(len(recentOrders)) / float64(7*restaurant.Capacity)
	basePopularity += math.Min(orderVolumeFactor, 1.0) * 0.3 // 30% weight to order volume

	// review engagement
	recentReviews := s.getRecentReviews(restaurant.ID, 7*24*time.Hour)
	reviewFactor := float64(len(recentReviews)) / float64(len(recentOrders))
	basePopularity += math.Min(reviewFactor, 1.0) * 0.2 // 20% weight to review engagement

	// menu variety
	menuVarietyScore := s.calculateMenuVarietyScore(restaurant)
	basePopularity += menuVarietyScore * 0.1 // 10% weight to menu variety

	return basePopularity
}

func (s *Simulator) calculateMenuVarietyScore(restaurant *models.Restaurant) float64 {
	if len(restaurant.MenuItems) == 0 {
		return 0.0
	}

	// count items by type
	typeCount := make(map[string]int)
	for _, itemID := range restaurant.MenuItems {
		if item := s.getMenuItem(itemID); item != nil {
			typeCount[item.Type]++
		}
	}

	// calculate variety score based on distribution
	varietyScore := float64(len(typeCount)) / 5.0 // Normalize by expected number of types

	// bonus for balanced menu
	balanced := true
	for _, count := range typeCount {
		if count < 2 || count > len(restaurant.MenuItems)/2 {
			balanced = false
			break
		}
	}
	if balanced {
		varietyScore *= 1.2
	}

	return math.Min(varietyScore, 1.0)
}

func (s *Simulator) calculatePopularityTrend(restaurant *models.Restaurant) float64 {
	// Get orders from last 30 days
	recentOrders := s.getRecentCompletedOrders(restaurant.ID, 30*24*time.Hour)
	if len(recentOrders) == 0 {
		return 0.0
	}

	// group orders by day
	dailyOrders := make(map[string]int)
	dailyRevenue := make(map[string]float64)

	for _, order := range recentOrders {
		day := order.OrderPlacedAt.Format("2006-01-02")
		dailyOrders[day]++
		dailyRevenue[day] += order.TotalAmount
	}

	// calculate trend using linear regression
	var days []string
	for day := range dailyOrders {
		days = append(days, day)
	}
	sort.Strings(days)

	var x []float64
	var y []float64

	for i, day := range days {
		x = append(x, float64(i))
		y = append(y, float64(dailyOrders[day]))
	}

	trend := s.calculateLinearRegression(x, y)

	// normalize trend to [-1, 1] range
	maxAbsTrend := float64(restaurant.Capacity) / 30.0 // Max expected daily change
	normalizedTrend := math.Max(-1.0, math.Min(1.0, trend/maxAbsTrend))

	// adjust trend based on revenue
	revenueChange := s.calculateRevenueGrowth(dailyRevenue, days)

	// combine order trend and revenue trend
	return normalizedTrend*0.6 + revenueChange*0.4
}

func (s *Simulator) calculateLinearRegression(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0

	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumXX += x[i] * x[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	return slope
}

func (s *Simulator) calculateRevenueGrowth(dailyRevenue map[string]float64, days []string) float64 {
	if len(days) < 2 {
		return 0
	}

	// compare first and last week averages
	var firstWeekRevenue, lastWeekRevenue float64
	firstWeekDays := math.Min(7, float64(len(days)/3))
	lastWeekDays := firstWeekDays

	for i, day := range days {
		if float64(i) < firstWeekDays {
			firstWeekRevenue += dailyRevenue[day]
		}
		if float64(i) >= float64(len(days))-lastWeekDays {
			lastWeekRevenue += dailyRevenue[day]
		}
	}

	firstWeekAvg := firstWeekRevenue / firstWeekDays
	lastWeekAvg := lastWeekRevenue / lastWeekDays

	if firstWeekAvg == 0 {
		return 0
	}

	growth := (lastWeekAvg - firstWeekAvg) / firstWeekAvg
	return math.Max(-1.0, math.Min(1.0, growth))
}

func (s *Simulator) analyzeTimeBasedDemand(restaurant *models.Restaurant) map[int]float64 {
	demandByHour := make(map[int]float64)

	// get recent orders
	recentOrders := s.getRecentCompletedOrders(restaurant.ID, 14*24*time.Hour)

	// calculate base demand by hour
	ordersByHour := make(map[int]int)
	totalOrders := 0

	for _, order := range recentOrders {
		hour := order.OrderPlacedAt.Hour()
		ordersByHour[hour]++
		totalOrders++
	}

	// calculate average demand per hour
	if totalOrders > 0 {
		averageOrdersPerHour := float64(totalOrders) / (24 * 14)

		for hour := 0; hour < 24; hour++ {
			actualOrders := float64(ordersByHour[hour]) / 14 // Average for this hour
			demandByHour[hour] = actualOrders / averageOrdersPerHour
		}
	}

	// adjust for day of week patterns
	s.adjustDemandForDayPatterns(restaurant, demandByHour)

	// adjust for seasonal patterns
	s.adjustDemandForSeasonalPatterns(restaurant, demandByHour)

	return demandByHour
}

func (s *Simulator) adjustDemandForDayPatterns(restaurant *models.Restaurant, demand map[int]float64) {
	// get day-specific multipliers
	dayMultipliers := map[time.Weekday]float64{
		time.Monday:    0.9,
		time.Tuesday:   0.9,
		time.Wednesday: 1.0,
		time.Thursday:  1.1,
		time.Friday:    1.3,
		time.Saturday:  1.4,
		time.Sunday:    1.2,
	}

	// adjust demand based on restaurant type
	for _, cuisine := range restaurant.Cuisines {
		switch strings.ToLower(cuisine) {
		case "breakfast", "cafe":
			// boost morning hours on weekdays
			for hour := 6; hour < 11; hour++ {
				demand[hour] *= 1.3
			}
		case "business", "lunch":
			// boost lunch hours on weekdays
			for hour := 11; hour < 15; hour++ {
				demand[hour] *= 1.4
			}
		case "bar", "pub":
			// boost evening hours, especially weekends
			for hour := 17; hour < 23; hour++ {
				demand[hour] *= dayMultipliers[time.Friday] * 1.2
				demand[hour] *= dayMultipliers[time.Saturday] * 1.3
			}
		}
	}
}

func (s *Simulator) adjustDemandForSeasonalPatterns(restaurant *models.Restaurant, demand map[int]float64) {
	month := s.CurrentTime.Month()

	// seasonal adjustments based on cuisine type
	for _, cuisine := range restaurant.Cuisines {
		switch strings.ToLower(cuisine) {
		case "ice cream", "frozen yogurt":
			if month >= time.June && month <= time.August {
				for hour := range demand {
					demand[hour] *= 1.5
				}
			}
		case "soup", "hot pot":
			if month <= time.February || month >= time.November {
				for hour := range demand {
					demand[hour] *= 1.3
				}
			}
		}
	}
}

func (s *Simulator) updateRestaurantStatusBasedOnReputation(restaurant *models.Restaurant, metrics models.ReputationMetrics) {
	// adjust restaurant capacity based on reputation
	capacityMultiplier := 1.0
	if metrics.BaseRating >= ReputationThresholds["excellent"] {
		capacityMultiplier = 1.2
	} else if metrics.BaseRating <= ReputationThresholds["poor"] {
		capacityMultiplier = 0.8
	}

	restaurant.Capacity = int(float64(restaurant.Capacity) * capacityMultiplier)

	// adjust prep time efficiency based on reliability score
	restaurant.PickupEfficiency = metrics.ReliabilityScore

	// update price tier based on price-quality score
	if metrics.PriceQualityScore >= 1.1 {
		restaurant.PriceTier = "premium"
	} else if metrics.PriceQualityScore <= 0.9 {
		restaurant.PriceTier = "budget"
	} else {
		restaurant.PriceTier = "standard"
	}
}

func (s *Simulator) getReviewsInWindow(restaurantID string, duration time.Duration) []models.Review {
	var windowReviews []models.Review
	cutoff := s.CurrentTime.Add(-duration)

	for _, review := range s.Reviews {
		if review.RestaurantID == restaurantID && review.CreatedAt.After(cutoff) {
			windowReviews = append(windowReviews, review)
		}
	}

	return windowReviews
}

func (s *Simulator) calculateWindowRating(reviews []models.Review) float64 {
	if len(reviews) == 0 {
		return 0
	}

	var sum float64
	for _, review := range reviews {
		sum += review.OverallRating
	}

	return sum / float64(len(reviews))
}

func (s *Simulator) calculateRecentAverageRating(restaurant *models.Restaurant) float64 {
	reviews := s.getReviewsInWindow(restaurant.ID, 30*24*time.Hour)
	return s.calculateWindowRating(reviews)
}

func (s *Simulator) getRecentReviews(restaurantID string, duration time.Duration) []models.Review {
	// calculate cutoff time
	cutoffTime := s.CurrentTime.Add(-duration)

	// store recent reviews
	var recentReviews []models.Review

	// track number of reviews by rating for anomaly detection
	ratingCounts := make(map[float64]int)

	// first pass: collect all reviews within time period
	for _, review := range s.Reviews {
		if review.RestaurantID == restaurantID &&
			review.CreatedAt.After(cutoffTime) &&
			!review.IsIgnored {

			// check for potential spam or fake reviews
			if s.isValidReview(review, &ratingCounts) {
				recentReviews = append(recentReviews, review)
			}
		}
	}

	// sort reviews by creation time (most recent first)
	sort.Slice(recentReviews, func(i, j int) bool {
		return recentReviews[i].CreatedAt.After(recentReviews[j].CreatedAt)
	})

	// analyse patterns for suspicious activity
	if suspiciousReviews := s.detectSuspiciousPatterns(recentReviews); len(suspiciousReviews) > 0 {
		// filter out suspicious reviews
		filteredReviews := make([]models.Review, 0, len(recentReviews))
		suspiciousMap := make(map[string]bool)
		for _, id := range suspiciousReviews {
			suspiciousMap[id] = true
		}

		for _, review := range recentReviews {
			if !suspiciousMap[review.ID] {
				filteredReviews = append(filteredReviews, review)
			}
		}
		recentReviews = filteredReviews
	}

	// weight reviews based on reliability factors
	weightedReviews := s.weightReviews(recentReviews)

	// sort weighted reviews by score and time
	sort.Slice(weightedReviews, func(i, j int) bool {
		if weightedReviews[i].weight == weightedReviews[j].weight {
			return weightedReviews[i].review.CreatedAt.After(weightedReviews[j].review.CreatedAt)
		}
		return weightedReviews[i].weight > weightedReviews[j].weight
	})

	// return the final filtered and sorted reviews
	result := make([]models.Review, len(weightedReviews))
	for i, wr := range weightedReviews {
		result[i] = wr.review
	}

	return result
}

type weightedReview struct {
	review models.Review
	weight float64
}

func (s *Simulator) isValidReview(review models.Review, ratingCounts *map[float64]int) bool {
	// check for extreme ratings
	if review.OverallRating < 1 || review.OverallRating > 5 {
		return false
	}

	// check for rating consistency
	if math.Abs(review.FoodRating-review.DeliveryRating) > 3 {
		// Suspicious if food and delivery ratings are very different
		return false
	}

	// track rating frequency
	(*ratingCounts)[review.OverallRating]++

	// check for suspicious review content
	if s.hasSpamIndicators(review.Comment) {
		return false
	}

	// verify order exists and was actually delivered
	order := s.getOrderByID(review.OrderID)
	if order == nil || order.Status != models.OrderStatusDelivered {
		return false
	}

	// check timing of review
	if review.CreatedAt.Before(order.ActualDeliveryTime) {
		return false
	}

	return true
}

func (s *Simulator) hasSpamIndicators(comment string) bool {
	if comment == "" {
		return false
	}

	comment = strings.ToLower(comment)

	// check for spam indicators
	spamIndicators := []string{
		"http://", "https://", "www.",
		"click here", "visit",
		"buy now", "discount",
		"$$", "£££", "€€€",
	}

	for _, indicator := range spamIndicators {
		if strings.Contains(comment, indicator) {
			return true
		}
	}

	// check for repetitive characters
	if strings.Contains(comment, strings.Repeat("!", 3)) ||
		strings.Contains(comment, strings.Repeat("?", 3)) {
		return true
	}

	// check for all caps
	upperCount := 0
	for _, r := range comment {
		if unicode.IsUpper(r) {
			upperCount++
		}
	}
	if float64(upperCount)/float64(len(comment)) > 0.7 {
		return true
	}

	return false
}

func (s *Simulator) detectSuspiciousPatterns(reviews []models.Review) []string {
	if len(reviews) < 2 {
		return nil
	}

	var suspiciousReviewIDs []string

	// group reviews by user
	userReviews := make(map[string][]models.Review)
	for _, review := range reviews {
		userReviews[review.CustomerID] = append(userReviews[review.CustomerID], review)
	}

	// check for suspicious patterns per user
	for _, reviews := range userReviews {
		// Too many reviews in short time
		if len(reviews) >= 3 {
			// check time between first and last review
			duration := reviews[len(reviews)-1].CreatedAt.Sub(reviews[0].CreatedAt)
			if duration < 1*time.Hour {
				// Mark all reviews from this user as suspicious
				for _, review := range reviews {
					suspiciousReviewIDs = append(suspiciousReviewIDs, review.ID)
				}
				continue
			}
		}

		// check for identical ratings/comments
		if len(reviews) >= 2 {
			identicalRatings := true
			identicalComments := true
			firstReview := reviews[0]

			for i := 1; i < len(reviews); i++ {
				if reviews[i].OverallRating != firstReview.OverallRating {
					identicalRatings = false
				}
				if reviews[i].Comment != firstReview.Comment {
					identicalComments = false
				}
			}

			if identicalRatings && identicalComments {
				// mark all but the first review as suspicious
				for i := 1; i < len(reviews); i++ {
					suspiciousReviewIDs = append(suspiciousReviewIDs, reviews[i].ID)
				}
			}
		}
	}

	// check for review bombing (many negative reviews in short time)
	timeWindow := 6 * time.Hour
	negativeReviews := 0
	for _, review := range reviews {
		if review.CreatedAt.After(s.CurrentTime.Add(-timeWindow)) && review.OverallRating <= 2 {
			negativeReviews++
		}
	}

	if negativeReviews >= 5 { // threshold for review bombing
		// mark recent negative reviews as suspicious
		for _, review := range reviews {
			if review.CreatedAt.After(s.CurrentTime.Add(-timeWindow)) && review.OverallRating <= 2 {
				suspiciousReviewIDs = append(suspiciousReviewIDs, review.ID)
			}
		}
	}

	return suspiciousReviewIDs
}

func (s *Simulator) weightReviews(reviews []models.Review) []weightedReview {
	weightedReviews := make([]weightedReview, len(reviews))

	for i, review := range reviews {
		weight := 1.0

		// time decay factor
		timeSinceReview := s.CurrentTime.Sub(review.CreatedAt)
		timeWeight := math.Exp(-timeSinceReview.Hours() / (30 * 24)) // 30-day half-life
		weight *= timeWeight

		// user reliability weight
		userWeight := s.calculateUserReliability(review.CustomerID)
		weight *= userWeight

		// order value weight
		if order := s.getOrderByID(review.OrderID); order != nil {
			orderWeight := s.calculateOrderWeight(order)
			weight *= orderWeight
		}

		// review quality weight
		qualityWeight := s.calculateReviewQuality(review)
		weight *= qualityWeight

		weightedReviews[i] = weightedReview{
			review: review,
			weight: weight,
		}
	}

	return weightedReviews
}

func (s *Simulator) calculateUserReliability(userID string) float64 {
	user := s.getUser(userID)
	if user == nil {
		return 0.5
	}

	reliability := 1.0

	// account age factor
	accountAge := s.CurrentTime.Sub(user.JoinDate)
	if accountAge < 7*24*time.Hour {
		reliability *= 0.7 // New accounts less reliable
	}

	// order history factor
	if user.LifetimeOrders > 10 {
		reliability *= 1.2
	} else if user.LifetimeOrders > 5 {
		reliability *= 1.1
	}

	// review history analysis
	userReviews := s.getUserReviews(userID)
	if len(userReviews) > 0 {
		// calculate rating variance
		var sum, sumSq float64
		for _, review := range userReviews {
			sum += review.OverallRating
			sumSq += review.OverallRating * review.OverallRating
		}
		mean := sum / float64(len(userReviews))
		variance := (sumSq / float64(len(userReviews))) - (mean * mean)

		// high variance might indicate less reliable reviews
		if variance > 2 {
			reliability *= 0.9
		}
	}

	return reliability
}

func (s *Simulator) calculateOrderWeight(order *models.Order) float64 {
	weight := 1.0

	// Order value factor
	if order.TotalAmount > 50 {
		weight *= 1.2
	} else if order.TotalAmount < 15 {
		weight *= 0.9
	}

	// Order complexity factor
	if len(order.Items) > 3 {
		weight *= 1.1
	}

	return weight
}

func (s *Simulator) calculateReviewQuality(review models.Review) float64 {
	weight := 1.0

	// Comment quality
	if review.Comment != "" {
		commentLength := len(strings.Split(review.Comment, " "))
		if commentLength > 20 {
			weight *= 1.2 // Detailed reviews weighted higher
		} else if commentLength < 5 {
			weight *= 0.9 // Very short reviews weighted lower
		}
	} else {
		weight *= 0.8 // No comment
	}

	// Rating consistency
	ratingDeviation := math.Abs(review.FoodRating - review.DeliveryRating)
	if ratingDeviation > 2 {
		weight *= 0.9 // Inconsistent ratings weighted lower
	}

	return weight
}

func (s *Simulator) getUserReviews(userID string) []models.Review {
	var userReviews []models.Review

	// create review metrics to help with analysis
	metrics := ReviewMetrics{
		ratingDistribution: make(map[float64]int),
		reviewFrequency:    make(map[string]int),
		commonPhrases:      make(map[string]int),
	}

	// first pass: collect reviews and basic metrics
	for _, review := range s.Reviews {
		if review.CustomerID == userID {
			// skip ignored reviews
			if review.IsIgnored {
				continue
			}

			// collect the review
			userReviews = append(userReviews, review)

			// update metrics
			metrics.totalReviews++
			metrics.averageRating += review.OverallRating
			metrics.ratingDistribution[review.OverallRating]++
			metrics.reviewTimes = append(metrics.reviewTimes, review.CreatedAt)

			// track review frequency by date
			dateKey := review.CreatedAt.Format("2006-01-02")
			metrics.reviewFrequency[dateKey]++

			// track common phrases in comments
			if review.Comment != "" {
				phrases := s.extractCommonPhrases(review.Comment)
				for _, phrase := range phrases {
					metrics.commonPhrases[phrase]++
				}
			}
		}
	}

	if metrics.totalReviews == 0 {
		return nil
	}

	metrics.averageRating /= float64(metrics.totalReviews)

	// analyse for suspicious patterns
	suspiciousReviews := s.analyzeSuspiciousUserReviews(userReviews, metrics)

	// filter out suspicious reviews
	if len(suspiciousReviews) > 0 {
		filteredReviews := make([]models.Review, 0, len(userReviews))
		suspiciousMap := make(map[string]bool)
		for _, id := range suspiciousReviews {
			suspiciousMap[id] = true
		}

		for _, review := range userReviews {
			if !suspiciousMap[review.ID] {
				filteredReviews = append(filteredReviews, review)
			}
		}
		userReviews = filteredReviews
	}

	// sort reviews by time (most recent first)
	sort.Slice(userReviews, func(i, j int) bool {
		return userReviews[i].CreatedAt.After(userReviews[j].CreatedAt)
	})

	return userReviews
}

func (s *Simulator) analyzeSuspiciousUserReviews(reviews []models.Review, metrics ReviewMetrics) []string {
	var suspiciousReviews []string

	// check for rapid-fire reviews
	if len(metrics.reviewTimes) >= 2 {
		sort.Slice(metrics.reviewTimes, func(i, j int) bool {
			return metrics.reviewTimes[i].Before(metrics.reviewTimes[j])
		})

		for i := 1; i < len(metrics.reviewTimes); i++ {
			timeDiff := metrics.reviewTimes[i].Sub(metrics.reviewTimes[i-1])
			if timeDiff < 5*time.Minute {
				// reviews too close together are suspicious
				for _, review := range reviews {
					if review.CreatedAt == metrics.reviewTimes[i] ||
						review.CreatedAt == metrics.reviewTimes[i-1] {
						suspiciousReviews = append(suspiciousReviews, review.ID)
					}
				}
			}
		}
	}

	// check for excessive reviews in one day
	for date, count := range metrics.reviewFrequency {
		if count > 5 { // more than 5 reviews in a day is suspicious
			for _, review := range reviews {
				if review.CreatedAt.Format("2006-01-02") == date {
					suspiciousReviews = append(suspiciousReviews, review.ID)
				}
			}
		}
	}

	// Check for rating patterns
	if metrics.totalReviews >= 5 {
		// Check for too many identical ratings
		for rating, count := range metrics.ratingDistribution {
			if float64(count)/float64(metrics.totalReviews) > 0.8 {
				// More than 80% identical ratings is suspicious
				for _, review := range reviews {
					if review.OverallRating == rating {
						suspiciousReviews = append(suspiciousReviews, review.ID)
					}
				}
			}
		}
	}

	// check for repeated content
	if len(metrics.commonPhrases) > 0 {
		for phrase, count := range metrics.commonPhrases {
			if count >= 3 && len(phrase) > 10 { // Same long phrase used 3+ times
				for _, review := range reviews {
					if strings.Contains(review.Comment, phrase) {
						suspiciousReviews = append(suspiciousReviews, review.ID)
					}
				}
			}
		}
	}

	// verify order history
	orderHistory := s.getOrdersByUser(reviews[0].CustomerID)
	orderMap := make(map[string]bool)
	for _, order := range orderHistory {
		orderMap[order.ID] = true
	}

	// check for reviews without matching orders
	for _, review := range reviews {
		if !orderMap[review.OrderID] {
			suspiciousReviews = append(suspiciousReviews, review.ID)
		}
	}

	return suspiciousReviews
}

func (s *Simulator) extractCommonPhrases(comment string) []string {
	var phrases []string

	// normalize text
	comment = strings.ToLower(comment)
	words := strings.Fields(comment)

	// etract 3-word phrases
	if len(words) >= 3 {
		for i := 0; i <= len(words)-3; i++ {
			phrase := strings.Join(words[i:i+3], " ")
			phrases = append(phrases, phrase)
		}
	}

	return phrases
}

func (s *Simulator) getOrdersByUser(userID string) []models.Order {
	var userOrders []models.Order

	// get orders from simulator's order history
	if orders, exists := s.OrdersByUser[userID]; exists {
		userOrders = orders
	} else {
		// fallback to searching all orders
		for _, order := range s.Orders {
			if order.CustomerID == userID {
				userOrders = append(userOrders, order)
			}
		}
	}

	// sort orders by time (most recent first)
	sort.Slice(userOrders, func(i, j int) bool {
		return userOrders[i].OrderPlacedAt.After(userOrders[j].OrderPlacedAt)
	})

	return userOrders
}

func (s *Simulator) analyzeMarketPosition(restaurant *models.Restaurant) MarketPosition {
	if restaurant == nil {
		return MarketPosition{}
	}

	// get nearby competing restaurants
	nearbyCompetitors := s.getNearbyRestaurants(restaurant.Location, 5.0) // 5km radius

	var competingRestaurants []*models.Restaurant
	for _, competitor := range nearbyCompetitors {
		if competitor.ID != restaurant.ID && s.hasCuisineOverlap(restaurant, competitor) {
			competingRestaurants = append(competingRestaurants, competitor)
		}
	}

	// calculate position metrics
	avgPrice := s.calculateAverageItemPrice(restaurant)
	avgCompetitorPrice := s.calculateAverageCompetitorPrice(competingRestaurants)

	// determine price tier
	var priceTier string
	priceRatio := avgPrice / avgCompetitorPrice
	switch {
	case priceRatio >= 1.3:
		priceTier = "premium"
	case priceRatio <= 0.8:
		priceTier = "budget"
	default:
		priceTier = "standard"
	}

	// determine quality tier
	var qualityTier string
	switch {
	case restaurant.Rating >= 4.5:
		qualityTier = "premium"
	case restaurant.Rating >= 4.0:
		qualityTier = "high"
	case restaurant.Rating >= 3.5:
		qualityTier = "standard"
	default:
		qualityTier = "budget"
	}

	// calculate relative popularity
	popularity := s.calculateRelativePopularity(restaurant, competingRestaurants)

	// calculate competitive position
	competitivePos := s.calculateCompetitivePosition(restaurant, competingRestaurants)

	return MarketPosition{
		PriceTier:      priceTier,
		QualityTier:    qualityTier,
		Popularity:     popularity,
		CompetitivePos: competitivePos,
	}
}

func (s *Simulator) calculateAverageCompetitorPrice(competitors []*models.Restaurant) float64 {
	if len(competitors) == 0 {
		return 0
	}

	var totalPrice float64
	var count int
	for _, competitor := range competitors {
		price := s.calculateAverageItemPrice(competitor)
		if price > 0 {
			totalPrice += price
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalPrice / float64(count)
}

func (s *Simulator) calculateRelativePopularity(restaurant *models.Restaurant, competitors []*models.Restaurant) float64 {
	if len(competitors) == 0 {
		return 1.0
	}

	// calculate orders per day for target restaurant
	targetOrders := float64(len(s.getRecentCompletedOrders(restaurant.ID, 24*time.Hour)))

	// calculate average orders per day for competitors
	var totalCompetitorOrders float64
	for _, competitor := range competitors {
		competitorOrders := float64(len(s.getRecentCompletedOrders(competitor.ID, 24*time.Hour)))
		totalCompetitorOrders += competitorOrders
	}
	avgCompetitorOrders := totalCompetitorOrders / float64(len(competitors))

	if avgCompetitorOrders == 0 {
		return 1.0
	}

	return targetOrders / avgCompetitorOrders
}

func (s *Simulator) calculateCompetitivePosition(restaurant *models.Restaurant, competitors []*models.Restaurant) float64 {
	if len(competitors) == 0 {
		return 1.0
	}

	// calculate metrics for target restaurant
	targetMetrics := CompetitiveMetrics{
		rating:  restaurant.Rating,
		price:   s.calculateAverageItemPrice(restaurant),
		volume:  float64(len(s.getRecentCompletedOrders(restaurant.ID, 7*24*time.Hour))),
		variety: float64(len(restaurant.MenuItems)),
	}

	// calculate metrics for competitors
	var competitorScores []float64
	for _, competitor := range competitors {
		competitorMetrics := CompetitiveMetrics{
			rating:  competitor.Rating,
			price:   s.calculateAverageItemPrice(competitor),
			volume:  float64(len(s.getRecentCompletedOrders(competitor.ID, 7*24*time.Hour))),
			variety: float64(len(competitor.MenuItems)),
		}

		// calculate relative score
		score := s.calculateCompetitiveScore(targetMetrics, competitorMetrics)
		competitorScores = append(competitorScores, score)
	}

	// calculate final position (-1 to 1, where positive means better than competition)
	var totalScore float64
	for _, score := range competitorScores {
		totalScore += score
	}

	return math.Max(-1.0, math.Min(1.0, totalScore/float64(len(competitors))))
}

func (s *Simulator) calculateCompetitiveScore(target, competitor CompetitiveMetrics) float64 {
	// weight factors
	const (
		ratingWeight  = 0.4
		priceWeight   = 0.3
		volumeWeight  = 0.2
		varietyWeight = 0.1
	)

	var score float64

	// rating comparison
	ratingDiff := (target.rating - competitor.rating) / 5.0
	score += ratingDiff * ratingWeight

	// price comparison (lower price is better, assuming similar quality)
	if target.price > 0 && competitor.price > 0 {
		priceDiff := (competitor.price - target.price) / competitor.price
		score += priceDiff * priceWeight
	}

	// volume comparison
	if competitor.volume > 0 {
		volumeDiff := (target.volume - competitor.volume) / math.Max(target.volume, competitor.volume)
		score += volumeDiff * volumeWeight
	}

	// variety comparison
	if competitor.variety > 0 {
		varietyDiff := (target.variety - competitor.variety) / math.Max(target.variety, competitor.variety)
		score += varietyDiff * varietyWeight
	}

	return score
}

func (s *Simulator) analyzeSegmentAppeal(restaurant *models.Restaurant) map[string]float64 {
	segmentAppeal := make(map[string]float64)

	// calculate base appeal for each segment
	for segment, profile := range models.DefaultCustomerSegments {
		appeal := s.calculateSegmentAppeal(restaurant, segment, profile)
		segmentAppeal[segment] = appeal
	}

	// normalise appeals
	totalAppeal := 0.0
	for _, appeal := range segmentAppeal {
		totalAppeal += appeal
	}

	if totalAppeal > 0 {
		for segment := range segmentAppeal {
			segmentAppeal[segment] /= totalAppeal
		}
	}

	return segmentAppeal
}

func (s *Simulator) calculateSegmentAppeal(restaurant *models.Restaurant, segment string, profile models.CustomerSegment) float64 {
	avgPrice := s.calculateAverageItemPrice(restaurant)

	// base appeal
	appeal := 1.0

	// price alignment
	priceAppeal := s.calculatePriceAppealForSegment(avgPrice, profile)
	appeal *= priceAppeal

	// quality alignment
	qualityAppeal := s.calculateQualityAppealForSegment(restaurant.Rating, segment)
	appeal *= qualityAppeal

	// cuisine preference alignment
	cuisineAppeal := s.calculateCuisineAppealForSegment(restaurant.Cuisines, profile)
	appeal *= cuisineAppeal

	// location convenience
	locationAppeal := s.calculateLocationAppealForSegment(restaurant.Location, segment)
	appeal *= locationAppeal

	return appeal
}

func (s *Simulator) calculatePriceAppealForSegment(price float64, profile models.CustomerSegment) float64 {
	if price == 0 {
		return 0
	}

	// calculate how well the price matches the segment's average spend
	priceRatio := price / profile.AvgSpend

	switch {
	case priceRatio > 1.5:
		return 0.5 // too expensive
	case priceRatio > 1.2:
		return 0.8 // slightly expensive
	case priceRatio > 0.8:
		return 1.0 // good match
	case priceRatio > 0.5:
		return 0.9 // good value
	default:
		return 0.7 // too cheap (might indicate quality concerns)
	}
}

func (s *Simulator) calculateQualityAppealForSegment(rating float64, segment string) float64 {
	switch segment {
	case "frequent":
		if rating >= 4.5 {
			return 1.2
		} else if rating >= 4.0 {
			return 1.0
		}
		return 0.8
	case "regular":
		if rating >= 4.0 {
			return 1.1
		} else if rating >= 3.5 {
			return 1.0
		}
		return 0.9
	case "occasional":
		if rating >= 3.5 {
			return 1.0
		}
		return 0.9
	}
	return 1.0
}

func (s *Simulator) calculateCuisineAppealForSegment(cuisines []string, profile models.CustomerSegment) float64 {
	if len(cuisines) == 0 {
		return 0.5
	}

	maxAppeal := 0.0
	for _, cuisine := range cuisines {
		if weight, exists := profile.CuisinePreferences[strings.ToLower(cuisine)]; exists {
			if weight > maxAppeal {
				maxAppeal = weight
			}
		}
	}

	if maxAppeal == 0 {
		return 0.8 // default appeal for cuisines not in preferences
	}
	return maxAppeal
}

func (s *Simulator) calculateLocationAppealForSegment(location models.Location, segment string) float64 {
	// different segments have different distance tolerances
	maxDistance := map[string]float64{
		"frequent":   5.0, // km
		"regular":    3.0,
		"occasional": 2.0,
	}[segment]

	// calculate distance from city center
	distanceFromCenter := s.calculateDistance(location, models.Location{
		Lat: s.Config.CityLat,
		Lon: s.Config.CityLon,
	})

	if distanceFromCenter > maxDistance {
		return 0.5 + (maxDistance/distanceFromCenter)*0.5
	}
	return 1.0
}

func (s *Simulator) calculatePriceAppeal(restaurant *models.Restaurant) float64 {
	if restaurant == nil {
		return 0.0
	}

	avgPrice := s.calculateAverageItemPrice(restaurant)
	if avgPrice == 0 {
		return 0.0
	}

	// get competitive price range
	competitors := s.getNearbyRestaurants(restaurant.Location, 5.0)
	var competitivePrices []float64
	for _, comp := range competitors {
		if comp.ID != restaurant.ID && s.hasCuisineOverlap(restaurant, comp) {
			price := s.calculateAverageItemPrice(comp)
			if price > 0 {
				competitivePrices = append(competitivePrices, price)
			}
		}
	}

	if len(competitivePrices) == 0 {
		return 1.0 // no direct competition
	}

	// calculate competitive metrics
	sort.Float64s(competitivePrices)
	medianPrice := competitivePrices[len(competitivePrices)/2]

	// calculate price position
	priceRatio := avgPrice / medianPrice
	qualityRatio := restaurant.Rating / 4.0 // normalize rating to 0-1 scale

	// price appeal is higher when quality justifies the price
	priceQualityBalance := qualityRatio / priceRatio

	// calculate price appeal based on market position
	var appeal float64
	switch {
	case priceRatio < 0.8:
		// significantly cheaper than competition
		appeal = 1.2 * math.Min(qualityRatio*1.2, 1.0)
	case priceRatio > 1.2:
		// significantly more expensive than competition
		appeal = 0.8 * math.Min(qualityRatio*1.5, 1.0)
	default:
		// competitive pricing
		appeal = 1.0 * math.Min(qualityRatio*1.3, 1.0)
	}

	// adjust appeal based on price-quality balance
	if priceQualityBalance > 1.2 {
		appeal *= 1.2 // great value for money
	} else if priceQualityBalance < 0.8 {
		appeal *= 0.8 // overpriced for quality
	}

	return math.Max(0.1, math.Min(2.0, appeal))
}

func (s *Simulator) calculateQualityAppeal(restaurant *models.Restaurant) float64 {
	if restaurant == nil {
		return 0.0
	}

	// base quality score from rating
	baseQuality := (restaurant.Rating - 3.0) / 2.0 // Normalize to 0-1 scale

	// recent reviews analysis
	recentReviews := s.getRecentReviews(restaurant.ID, 30*24*time.Hour)

	var recentRatingSum float64
	var recentRatingCount int
	var reviewQuality float64

	for _, review := range recentReviews {
		recentRatingSum += review.FoodRating
		recentRatingCount++

		// consider review quality (detailed reviews weighted more)
		if len(review.Comment) > 50 {
			reviewQuality += 0.2
		} else if len(review.Comment) > 20 {
			reviewQuality += 0.1
		}
	}

	// calculate recent rating trend
	var recentTrend float64
	if recentRatingCount > 0 {
		recentAvg := recentRatingSum / float64(recentRatingCount)
		recentTrend = (recentAvg - restaurant.Rating) / restaurant.Rating
	}

	// food consistency score
	consistencyScore := s.calculateConsistencyAppeal(restaurant)

	// menu quality indicators
	menuQuality := s.calculateMenuQuality(restaurant)

	// combine all factors
	qualityScore := baseQuality*0.4 + // 40% weight to base rating
		recentTrend*0.2 + // 20% weight to recent trend
		reviewQuality*0.1 + // 10% weight to review quality
		consistencyScore*0.2 + // 20% weight to consistency
		menuQuality*0.1 // 10% weight to menu quality

	return math.Max(0.0, math.Min(1.0, qualityScore+0.5)) // Normalize to 0.5-1.5 range
}

func (s *Simulator) calculateConsistencyAppeal(restaurant *models.Restaurant) float64 {
	if restaurant == nil {
		return 0.0
	}

	// analyse recent orders and reviews
	recentOrders := s.getRecentCompletedOrders(restaurant.ID, 30*24*time.Hour)
	recentReviews := s.getRecentReviews(restaurant.ID, 30*24*time.Hour)

	if len(recentOrders) == 0 || len(recentReviews) == 0 {
		return 0.5 // default consistency score
	}

	// calculate rating variance
	var ratingSum, ratingSquaredSum float64
	var ratingCount int

	for _, review := range recentReviews {
		ratingSum += review.FoodRating
		ratingSquaredSum += review.FoodRating * review.FoodRating
		ratingCount++
	}

	if ratingCount == 0 {
		return 0.5
	}

	meanRating := ratingSum / float64(ratingCount)
	variance := (ratingSquaredSum / float64(ratingCount)) - (meanRating * meanRating)

	// lower variance indicates higher consistency
	consistencyScore := 1.0 - math.Min(1.0, variance/2.0)

	// analyse delivery time consistency
	var deliveryVariance float64
	if len(recentOrders) > 1 {
		deliveryVariance = s.calculateDeliveryTimeVariance(recentOrders)
		// convert variance to a score (lower variance = higher score)
		deliveryConsistency := 1.0 - math.Min(1.0, deliveryVariance/3600) // Normalize to hours
		consistencyScore = (consistencyScore + deliveryConsistency) / 2.0
	}

	return consistencyScore
}

func (s *Simulator) calculateMenuQuality(restaurant *models.Restaurant) float64 {
	if len(restaurant.MenuItems) == 0 {
		return 0.0
	}

	var totalQuality float64
	typeCount := make(map[string]int)

	for _, itemID := range restaurant.MenuItems {
		item := s.getMenuItem(itemID)
		if item == nil {
			continue
		}

		// track menu item types
		typeCount[item.Type]++

		// individual item quality based on popularity and prep complexity
		itemQuality := (item.Popularity + item.PrepComplexity) / 2.0
		totalQuality += itemQuality
	}

	// calculate average item quality
	avgQuality := totalQuality / float64(len(restaurant.MenuItems))

	// menu balance score
	balanceScore := s.calculateMenuBalance(typeCount)

	// combine scores
	return avgQuality*0.7 + balanceScore*0.3
}

func (s *Simulator) calculateMenuBalance(typeCount map[string]int) float64 {
	if len(typeCount) == 0 {
		return 0.0
	}

	expectedTypes := map[string]float64{
		"main course": 0.4, // 40% main courses
		"appetizer":   0.2, // 20% appetizers
		"side dish":   0.2, // 20% sides
		"dessert":     0.1, // 10% desserts
		"drink":       0.1, // 10% drinks
	}

	totalItems := 0
	for _, count := range typeCount {
		totalItems += count
	}

	var balanceScore float64
	for itemType, expectedRatio := range expectedTypes {
		actualRatio := float64(typeCount[itemType]) / float64(totalItems)
		difference := math.Abs(actualRatio - expectedRatio)
		balanceScore += 1.0 - difference
	}

	return balanceScore / float64(len(expectedTypes))
}

func (s *Simulator) getUserPreviousRating(userID string, restaurantID string) float64 {
	// Look through user's reviews for this restaurant
	for _, review := range s.Reviews {
		if review.CustomerID == userID &&
			review.RestaurantID == restaurantID &&
			!review.IsIgnored {
			return review.OverallRating
		}
	}
	return 0 // No previous rating
}

func (s *Simulator) calculateTimeBasedEventMultiplier() float64 {
	multiplier := 1.0

	// Pay day effect (assume 1st and 15th of month)
	dayOfMonth := s.CurrentTime.Day()
	if dayOfMonth == 1 || dayOfMonth == 15 || dayOfMonth == 14 || dayOfMonth == 16 {
		multiplier *= 1.2
	}

	// Weekend effect
	if s.CurrentTime.Weekday() == time.Friday || s.CurrentTime.Weekday() == time.Saturday {
		multiplier *= 1.3
	}

	// End of month effect
	if dayOfMonth >= 28 {
		multiplier *= 0.9 // People tend to spend less at month end
	}

	return multiplier
}

func (s *Simulator) calculateHolidayMultiplier() float64 {
	dateStr := s.CurrentTime.Format("01-02")

	holidayMultipliers := map[string]float64{
		"12-24": 1.5, // Christmas Eve
		"12-25": 0.3, // Christmas Day
		"12-31": 2.0, // New Year's Eve
		"01-01": 1.8, // New Year's Day
		"02-14": 1.7, // Valentine's Day
		"07-04": 0.7, // Independence Day
		"11-25": 0.3, // Thanksgiving
	}

	if multiplier, exists := holidayMultipliers[dateStr]; exists {
		return multiplier
	}

	return 1.0
}

func (s *Simulator) calculateDeliveryTimeVariance(orders []models.Order) float64 {
	if len(orders) < 2 {
		return 0
	}

	var sum, sumSq float64
	count := 0

	for _, order := range orders {
		if !order.ActualDeliveryTime.IsZero() && !order.OrderPlacedAt.IsZero() {
			deliveryTime := order.ActualDeliveryTime.Sub(order.OrderPlacedAt).Seconds()
			sum += deliveryTime
			sumSq += deliveryTime * deliveryTime
			count++
		}
	}

	if count < 2 {
		return 0
	}

	mean := sum / float64(count)
	variance := (sumSq / float64(count)) - (mean * mean)

	return variance
}
