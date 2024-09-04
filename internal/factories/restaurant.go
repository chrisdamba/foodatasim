package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
)

type RestaurantFactory struct{}

func (rf *RestaurantFactory) CreateRestaurant(config models.Config) models.Restaurant {

	// Use config to determine location within the city bounds
	latRange := config.UrbanRadius / 111.0 // Approx. conversion from km to degrees
	lonRange := latRange / math.Cos(config.CityLat*math.Pi/180.0)

	lat := fake.Float64(6, int(config.CityLat-latRange), int(config.CityLat+latRange))
	lon := fake.Float64(6, int(config.CityLon-lonRange), int(config.CityLon+lonRange))

	// Use config for time-related fields
	avgPrepTime := fake.Float64(0, config.MinPrepTime, config.MaxPrepTime)

	return models.Restaurant{
		ID:             cuid.New(),
		Host:           fake.Internet().Domain(),
		Name:           fake.Company().Name(),
		Currency:       1, // Assuming 1 represents the default currency
		Phone:          fake.Phone().Number(),
		Town:           fake.Address().City(),
		SlugName:       fake.Internet().Slug(),
		WebsiteLogoURL: fake.Internet().URL(),
		Offline:        "DISABLED",
		Location: models.Location{
			Lat: lat,
			Lon: lon,
		},
		Cuisines:         generateRandomCuisines(),
		Rating:           fake.Float64(1, 1, 5),
		TotalRatings:     fake.Float64(0, 0, 1000),
		PrepTime:         fake.Float64(0, 10, 60),
		MinPrepTime:      fake.Float64(0, config.MinPrepTime, int(avgPrepTime)),
		AvgPrepTime:      fake.Float64(0, 15, 45),
		PickupEfficiency: fake.Float64(2, 50, 150) / 100,
		Capacity:         fake.IntBetween(10, 50),
		MenuItems:        []string{},
		CurrentOrders:    []models.Order{},
	}
}

func generateRandomCuisines() []string {
	allCuisines := []string{"Italian", "Indian", "American", "Japanese", "Mexican", "Chinese", "Thai", "Greek", "French", "Mediterranean"}
	cuisineCount := rand.Intn(3) + 1 // 1 to 3 cuisines
	cuisines := make([]string, cuisineCount)
	for i := 0; i < cuisineCount; i++ {
		cuisines[i] = allCuisines[rand.Intn(len(allCuisines))]
	}
	return cuisines
}
