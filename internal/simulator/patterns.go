package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"math"
	"time"
)

var (
	RestaurantPatterns = map[string]RestaurantPattern{
		"fast_food": {
			CuisineType: "fast_food",
			PeakHours: map[time.Weekday][]int{
				time.Friday:   {11, 12, 13, 18, 19, 20, 21, 22, 23},
				time.Saturday: {12, 13, 14, 18, 19, 20, 21, 22, 23},
				time.Sunday:   {12, 13, 14, 18, 19, 20},
			},
			PrepTimeRange: TimeRange{
				Min: 5,
				Max: 20,
				Std: 3,
			},
			PriceCategory:    "low",
			PopularityFactor: 1.3,
		},
		"fine_dining": {
			CuisineType: "fine_dining",
			PeakHours: map[time.Weekday][]int{
				time.Friday:   {19, 20, 21},
				time.Saturday: {19, 20, 21},
			},
			PrepTimeRange: TimeRange{
				Min: 20,
				Max: 45,
				Std: 5,
			},
			PriceCategory:    "high",
			PopularityFactor: 0.7,
			SeasonalItems: map[time.Month][]string{
				time.December: {"Christmas_Special", "Winter_Menu"},
				time.February: {"Valentine_Special"},
				time.July:     {"Summer_Menu"},
			},
		},
	}

	RestaurantClusters = map[string]RestaurantCluster{
		"premium": {
			Type:                "premium",
			BaseCapacity:        30,
			CapacityFlexibility: 0.2,
			QualityVariance:     0.1,
			PriceTier:           3,
			PreparationSpeed:    0.8,
		},
		"standard": {
			Type:                "standard",
			BaseCapacity:        50,
			CapacityFlexibility: 0.3,
			QualityVariance:     0.15,
			PriceTier:           2,
			PreparationSpeed:    1.0,
		},
		"budget": {
			Type:                "budget",
			BaseCapacity:        80,
			CapacityFlexibility: 0.4,
			QualityVariance:     0.2,
			PriceTier:           1,
			PreparationSpeed:    1.2,
		},
	}
)

var (
	DefaultOrderPatterns = map[string]OrderPattern{
		"breakfast_rush": {
			Type:            "breakfast",
			BaseProbability: 0.4,
			TimeMultipliers: map[int]float64{
				7:  1.5,
				8:  2.0,
				9:  1.8,
				10: 1.2,
			},
			WeatherEffects: map[string]float64{
				"rain":   1.2,
				"cold":   1.3,
				"normal": 1.0,
				"hot":    0.9,
			},
			MenuPreferences: map[string]float64{
				"breakfast": 2.0,
				"coffee":    2.5,
				"pastries":  1.8,
			},
		},
		"lunch_rush": {
			Type:            "lunch",
			BaseProbability: 0.6,
			TimeMultipliers: map[int]float64{
				11: 1.3,
				12: 2.0,
				13: 2.0,
				14: 1.5,
			},
			WeekdayMultipliers: map[time.Weekday]float64{
				time.Monday:   1.2,
				time.Friday:   1.4,
				time.Saturday: 0.8,
				time.Sunday:   0.7,
			},
		},
		"dinner_rush": {
			Type:            "dinner",
			BaseProbability: 0.5,
			TimeMultipliers: map[int]float64{
				17: 1.2,
				18: 1.8,
				19: 2.0,
				20: 1.7,
				21: 1.3,
			},
			WeekdayMultipliers: map[time.Weekday]float64{
				time.Friday:   1.6,
				time.Saturday: 1.5,
				time.Sunday:   1.3,
			},
		},
		"late_night": {
			Type:            "late_night",
			BaseProbability: 0.3,
			TimeMultipliers: map[int]float64{
				22: 1.4,
				23: 1.6,
				0:  1.3,
				1:  1.0,
				2:  0.8,
			},
			WeekdayMultipliers: map[time.Weekday]float64{
				time.Friday:   2.0,
				time.Saturday: 1.8,
			},
			MenuPreferences: map[string]float64{
				"fast_food": 1.8,
				"pizza":     2.0,
				"snacks":    1.5,
			},
		},
	}

	MenuTimePatterns = map[string]MenuTimePattern{
		"breakfast": {
			ItemType:  "breakfast",
			PeakHours: []int{7, 8, 9, 10},
			DayPartPreference: map[string]float64{
				"morning":   2.0,
				"afternoon": 0.3,
				"evening":   0.1,
				"night":     0.2,
			},
		},
		"coffee": {
			ItemType:  "coffee",
			PeakHours: []int{7, 8, 9, 15, 16},
			WeatherPreference: map[string]float64{
				"cold":   1.5,
				"rain":   1.3,
				"normal": 1.0,
				"hot":    0.7,
			},
		},
		"ice_cream": {
			ItemType: "dessert",
			SeasonalMonths: []time.Month{
				time.June, time.July, time.August,
			},
			WeatherPreference: map[string]float64{
				"hot":    2.0,
				"normal": 1.0,
				"cold":   0.3,
				"rain":   0.5,
			},
		},
	}
)

var (
	ReviewPatterns = map[string]ReviewPattern{
		"frequent": {
			UserSegment:     "frequent",
			BaseProbability: 0.4,
			RatingBias: RatingBias{
				FoodBase:         4.2,
				DeliveryBase:     4.0,
				TimeInfluence:    0.8,
				PriceInfluence:   0.6,
				WeatherInfluence: 0.4,
			},
			CommentPatterns: []CommentPattern{
				{
					Sentiment: "positive",
					Templates: []string{
						"Consistently good food and delivery.",
						"Another great experience!",
						"Regular customer, never disappointed.",
					},
					Triggers: map[string]float64{
						"late_delivery":  -15, // minutes late
						"early_delivery": 5,   // minutes early
						"high_price":     50,  // price threshold
					},
				},
				{
					Sentiment: "negative",
					Templates: []string{
						"Usually better than this.",
						"Not up to usual standards.",
						"Disappointed this time.",
					},
				},
			},
		},
		"regular": {
			UserSegment:     "regular",
			BaseProbability: 0.3,
			RatingBias: RatingBias{
				FoodBase:         4.0,
				DeliveryBase:     3.8,
				TimeInfluence:    0.6,
				PriceInfluence:   0.8,
				WeatherInfluence: 0.5,
			},
		},
		"occasional": {
			UserSegment:     "occasional",
			BaseProbability: 0.2,
			RatingBias: RatingBias{
				FoodBase:         3.8,
				DeliveryBase:     3.5,
				TimeInfluence:    0.4,
				PriceInfluence:   1.0,
				WeatherInfluence: 0.6,
			},
		},
	}

	TimeBasedReviewPatterns = map[string]float64{
		"breakfast":  1.0,
		"lunch":      1.2,
		"dinner":     1.3,
		"late_night": 0.8,
	}

	WeatherImpact = map[string]struct {
		RatingAdjustment  float64
		ReviewProbability float64
	}{
		"rain":   {-0.2, 1.2},
		"snow":   {-0.3, 1.3},
		"hot":    {-0.1, 1.1},
		"normal": {0.0, 1.0},
	}
)

var weatherTransitions = map[string][]WeatherTransition{
	"clear": {
		{Condition: "clear", BaseProbability: 0.6, MinDuration: 2 * time.Hour, MaxDuration: 12 * time.Hour},
		{Condition: "cloudy", BaseProbability: 0.3, MinDuration: 1 * time.Hour, MaxDuration: 8 * time.Hour},
		{Condition: "overcast", BaseProbability: 0.1, MinDuration: 2 * time.Hour, MaxDuration: 10 * time.Hour},
	},
	"cloudy": {
		{Condition: "clear", BaseProbability: 0.3, MinDuration: 1 * time.Hour, MaxDuration: 6 * time.Hour},
		{Condition: "cloudy", BaseProbability: 0.4, MinDuration: 2 * time.Hour, MaxDuration: 8 * time.Hour},
		{Condition: "overcast", BaseProbability: 0.2, MinDuration: 2 * time.Hour, MaxDuration: 10 * time.Hour},
		{Condition: "rain", BaseProbability: 0.1, MinDuration: 1 * time.Hour, MaxDuration: 4 * time.Hour},
	},
	"overcast": {
		{Condition: "cloudy", BaseProbability: 0.3, MinDuration: 2 * time.Hour, MaxDuration: 8 * time.Hour},
		{Condition: "overcast", BaseProbability: 0.3, MinDuration: 3 * time.Hour, MaxDuration: 12 * time.Hour},
		{Condition: "rain", BaseProbability: 0.3, MinDuration: 1 * time.Hour, MaxDuration: 6 * time.Hour},
		{Condition: "storm", BaseProbability: 0.1, MinDuration: 1 * time.Hour, MaxDuration: 3 * time.Hour},
	},
}

var seasonalModifiers = map[string]map[string]float64{
	"summer": {
		"clear": 1.3,
		"rain":  0.8,
		"storm": 1.2,
		"snow":  0.0,
	},
	"winter": {
		"clear": 0.7,
		"rain":  0.9,
		"storm": 0.7,
		"snow":  1.5,
	},
}

func calculateGrowthRate(baseRate float64, currentTime time.Time) float64 {
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

func calculateOrderProbability(currentTime time.Time, segment models.CustomerSegment) float64 {
	hour := currentTime.Hour()

	// Base probability from segment's orders per month
	baseProbability := segment.OrdersPerMonth / (30 * 24.0)

	// Time-based multipliers
	multiplier := 1.0

	// Weekend multiplier
	if currentTime.Weekday() == time.Saturday || currentTime.Weekday() == time.Sunday {
		multiplier *= 1.5
	}

	// Friday night multiplier
	if currentTime.Weekday() == time.Friday && hour >= 18 {
		multiplier *= 1.8
	}

	// Peak hours multiplier
	if isWeekdayPeakHour(hour) {
		multiplier *= segment.PeakHourBias * 1.5
	}

	return baseProbability * multiplier
}

func isWeekdayPeakHour(hour int) bool {
	weekdayPeakHours := map[int]bool{
		12: true, 13: true, // Lunch peak
		19: true, 20: true, // Dinner peak
	}
	return weekdayPeakHours[hour]
}
