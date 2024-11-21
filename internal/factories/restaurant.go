package factories

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
	"strings"
	"sync"
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
		Cuisines:         generateRandomCuisines(),
		Rating:           fake.Float64(1, 1, 5),
		TotalRatings:     fake.Float64(0, 0, 1000),
		PrepTime:         fake.Float64(0, 10, 60),
		MinPrepTime:      fake.Float64(0, config.MinPrepTime, int(avgPrepTime)),
		AvgPrepTime:      fake.Float64(0, 15, 45),
		PickupEfficiency: fake.Float64(2, 50, 150) / 100,
		Capacity:         fake.IntBetween(10, 50),
		MenuItems:        make([]string, 0),
		CurrentOrders:    []models.Order{},
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
