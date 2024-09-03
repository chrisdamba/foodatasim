package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"math/rand"
)

var fake = faker.New()

type UserFactory struct{}

func (uf *UserFactory) CreateUser(config models.Config) models.User {
	return models.User{
		ID:       cuid.New(),
		Name:     fake.Person().Name(),
		JoinDate: fake.Time().TimeBetween(config.StartDate.AddDate(-1, 0, 0), config.StartDate),
		Location: models.Location{
			Lat: fake.Float64(6, -90, 90),
			Lon: fake.Float64(6, -180, 180),
		},
		Preferences:         generateRandomPreferences(),
		DietaryRestrictions: generateRandomDietaryRestrictions(),
		OrderFrequency:      fake.Float64(2, 0, 100) / 100 * config.OrderFrequency,
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
