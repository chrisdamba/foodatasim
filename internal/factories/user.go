package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
)

var fake = faker.New()

type UserFactory struct{}

func (uf *UserFactory) CreateUser(config *models.Config) *models.User {
	// calculate city bounds
	latRange := config.UrbanRadius / 111.0 // Approx. conversion from km to degrees
	lonRange := latRange / math.Cos(config.CityLat*math.Pi/180.0)

	// generate random offsets within the urban radius
	latOffset := (rand.Float64()*2 - 1) * latRange
	lonOffset := (rand.Float64()*2 - 1) * lonRange

	// calculate final latitude and longitude
	lat := config.CityLat + latOffset
	lon := config.CityLon + lonOffset

	return &models.User{
		ID:       cuid.New(),
		Name:     fake.Person().Name(),
		JoinDate: fake.Time().TimeBetween(config.StartDate.AddDate(-1, 0, 0), config.StartDate),
		Location: models.Location{
			Lat: lat,
			Lon: lon,
		},
		Preferences:         generateRandomPreferences(),
		DietaryRestrictions: generateRandomDietaryRestrictions(),
		OrderFrequency:      fake.Float64(2, 50, 100) / 100 * config.OrderFrequency,
	}
}

func generateRandomPreferences() []string {
	allCuisines := []string{"Italian", "Indian", "Chinese", "Mexican", "Japanese", "Thai", "American", "French", "Greek", "Spanish", "Pizza", "Curry", "Burgers", "Sushi", "Tacos", "Pasta", "Salad", "Steak", "Seafood"}
	prefCount := rand.Intn(3) + 1 // 1 to 3 preferences
	preferences := make([]string, prefCount)
	for i := 0; i < prefCount; i++ {
		preferences[i] = allCuisines[rand.Intn(len(allCuisines))]
	}
	return preferences
}

func generateRandomDietaryRestrictions() []string {
	restrictions := []string{"Vegetarian", "Vegan", "Gluten-free", "Dairy-free", "Nut-free", "Halal", "Kosher"}
	restrictCount := fake.IntBetween(0, 2)
	if restrictCount == 0 {
		return nil
	}
	return []string{restrictions[rand.Intn(len(restrictions))]}
}
