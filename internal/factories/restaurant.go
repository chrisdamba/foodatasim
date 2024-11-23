package factories

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type RestaurantFactory struct {
	slugCache sync.Map // to track used slugs
}

func (rf *RestaurantFactory) CreateRestaurant(config *models.Config) *models.Restaurant {
	latRange := config.UrbanRadius / 111.0
	lonRange := latRange / math.Cos(config.CityLat*math.Pi/180.0)

	latOffset := (rand.Float64()*2 - 1) * latRange
	lonOffset := (rand.Float64()*2 - 1) * lonRange

	lat := config.CityLat + latOffset
	lon := config.CityLon + lonOffset

	avgPrepTime := fake.Float64(0, config.MinPrepTime, config.MaxPrepTime)

	name := fake.Company().Name()

	// generate reputation metrics
	reputationMetrics := models.ReputationMetrics{
		BaseRating:        rand.Float64(),
		ConsistencyScore:  rand.Float64(),
		TrendScore:        rand.Float64(),
		ReliabilityScore:  rand.Float64(),
		ResponseScore:     rand.Float64(),
		PriceQualityScore: rand.Float64(),
		LastUpdate:        time.Now(),
	}

	// generate market position
	priceTiers := []string{"budget", "standard", "premium"}
	qualityTiers := []string{"basic", "standard", "premium"}
	marketPosition := models.MarketPosition{
		PriceTier:      priceTiers[rand.Intn(len(priceTiers))],
		QualityTier:    qualityTiers[rand.Intn(len(qualityTiers))],
		Popularity:     rand.Float64(),
		CompetitivePos: rand.Float64(),
	}

	// generate popularity metrics
	popularityMetrics := models.RestaurantPopularityMetrics{
		BasePopularity:    rand.Float64(),
		TrendFactor:       rand.Float64(),
		TimeBasedDemand:   generateTimeBasedDemand(),
		CustomerSegments:  generateCustomerSegmentPreferences(),
		PriceAppeal:       rand.Float64(),
		QualityAppeal:     rand.Float64(),
		ConsistencyAppeal: rand.Float64(),
	}

	return &models.Restaurant{
		ID:             cuid.New(),
		Host:           fake.Internet().Domain(),
		Name:           name,
		Currency:       1,
		Phone:          fake.Phone().Number(),
		Town:           fake.Address().City(),
		SlugName:       rf.createUniqueSlug(name),
		WebsiteLogoURL: fake.Internet().URL(),
		Offline:        "DISABLED",
		Location: models.Location{
			Lat: lat,
			Lon: lon,
		},
		Cuisines:          generateRandomCuisines(),
		Rating:            fake.Float64(1, 1, 5),
		TotalRatings:      fake.Float64(0, 0, 1000),
		PrepTime:          fake.Float64(0, 10, 60),
		MinPrepTime:       fake.Float64(0, config.MinPrepTime, int(avgPrepTime)),
		AvgPrepTime:       fake.Float64(0, 15, 45),
		PickupEfficiency:  fake.Float64(2, 50, 150) / 100,
		Capacity:          fake.IntBetween(10, 50),
		MenuItems:         make([]string, 0),
		CurrentOrders:     []models.Order{},
		PriceTier:         marketPosition.PriceTier,
		ReputationMetrics: reputationMetrics,
		ReputationHistory: make([]models.ReputationMetrics, 0),
		DemandPatterns:    generateDemandPatterns(),
		MarketPosition:    marketPosition,
		PopularityMetrics: popularityMetrics,
	}
}

func (rf *RestaurantFactory) createUniqueSlug(name string) string {
	base := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, base)

	slug := base
	counter := 1

	for {
		if _, exists := rf.slugCache.LoadOrStore(slug, true); !exists {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, counter)
		counter++
	}
}

func generateRandomCuisines() []string {
	allCuisines := []string{"Italian", "Cafe", "Indian", "American", "European", "Japanese", "Mexican", "Native American", "Carribean", "Contemporary", "Continental", "Chinese", "Thai", "Vietnamese", "Greek", "French", "Mediterranean", "Moroccan", "Fast Food", "Street Food", "Homemade"}
	cuisineCount := rand.Intn(4) + 1 // 1 to 4 cuisines
	cuisines := make([]string, cuisineCount)
	for i := 0; i < cuisineCount; i++ {
		cuisines[i] = allCuisines[rand.Intn(len(allCuisines))]
	}
	return cuisines
}

func generateTimeBasedDemand() map[int]float64 {
	demand := make(map[int]float64)
	for hour := 0; hour < 24; hour++ {
		// Higher demand during lunch (11-14) and dinner (18-21) hours
		switch {
		case hour >= 11 && hour <= 14:
			demand[hour] = 0.6 + rand.Float64()*0.4 // 0.6-1.0
		case hour >= 18 && hour <= 21:
			demand[hour] = 0.7 + rand.Float64()*0.3 // 0.7-1.0
		default:
			demand[hour] = 0.1 + rand.Float64()*0.3 // 0.1-0.4
		}
	}
	return demand
}

func generateCustomerSegmentPreferences() map[string]float64 {
	return map[string]float64{
		"frequent":   0.4 + rand.Float64()*0.4, // 0.4-0.8
		"regular":    0.5 + rand.Float64()*0.3, // 0.5-0.8
		"occasional": 0.3 + rand.Float64()*0.4, // 0.3-0.7
	}
}

func generateDemandPatterns() map[int]float64 {
	patterns := make(map[int]float64)
	for hour := 0; hour < 24; hour++ {
		patterns[hour] = rand.Float64()
	}
	return patterns
}
