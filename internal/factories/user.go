package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jaswdr/faker"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
	"time"
)

var fake = faker.New()

type UserFactory struct{}

func (uf *UserFactory) assignUserSegment() string {
	r := rand.Float64()

	if r < models.DefaultCustomerSegments["frequent"].Ratio {
		return "frequent"
	} else if r < models.DefaultCustomerSegments["frequent"].Ratio+models.DefaultCustomerSegments["regular"].Ratio {
		return "regular"
	}
	return "occasional"
}

func (uf *UserFactory) createUserBehaviorProfile(segment string) models.UserBehaviourProfile {
	profile := models.UserBehaviourProfile{
		OrderTimingPreference: make(map[time.Weekday][]int),
		CuisineWeights:        make(map[string]float64),
		PaymentPreferences: map[string]float64{
			"card":   0.6,
			"cash":   0.3,
			"wallet": 0.1,
		},
	}

	switch segment {
	case "frequent":
		profile.PriceThresholds.Min = 15
		profile.PriceThresholds.Max = 80
		profile.PriceThresholds.Target = 50
		profile.OrderSizePreference = struct {
			Min     int `json:"min"`
			Max     int `json:"max"`
			Typical int `json:"typical"`
		}(struct {
			Min     int
			Max     int
			Typical int
		}{2, 5, 3})
		profile.LocationPreference.MaxDistance = 5.0

		// more varied timing preferences
		for day := time.Sunday; day <= time.Saturday; day++ {
			profile.OrderTimingPreference[day] = []int{12, 13, 19, 20, 21}
		}
		// additional late night preference for weekends
		profile.OrderTimingPreference[time.Friday] = append(
			profile.OrderTimingPreference[time.Friday],
			[]int{22, 23}...,
		)
		profile.OrderTimingPreference[time.Saturday] = append(
			profile.OrderTimingPreference[time.Saturday],
			[]int{22, 23}...,
		)

	case "regular":
		profile.PriceThresholds.Min = 10
		profile.PriceThresholds.Max = 50
		profile.PriceThresholds.Target = 35
		profile.OrderSizePreference = struct {
			Min     int `json:"min"`
			Max     int `json:"max"`
			Typical int `json:"typical"`
		}(struct {
			Min     int
			Max     int
			Typical int
		}{1, 4, 2})
		profile.LocationPreference.MaxDistance = 3.0

		// standard timing preferences
		for day := time.Sunday; day <= time.Saturday; day++ {
			if day >= time.Monday && day <= time.Friday {
				profile.OrderTimingPreference[day] = []int{12, 13, 19, 20}
			} else {
				profile.OrderTimingPreference[day] = []int{13, 14, 19, 20, 21}
			}
		}

	case "occasional":
		profile.PriceThresholds.Min = 8
		profile.PriceThresholds.Max = 30
		profile.PriceThresholds.Target = 20
		profile.OrderSizePreference = struct {
			Min     int `json:"min"`
			Max     int `json:"max"`
			Typical int `json:"typical"`
		}(struct {
			Min     int
			Max     int
			Typical int
		}{1, 3, 1})
		profile.LocationPreference.MaxDistance = 2.0

		// limited timing preferences
		for day := time.Sunday; day <= time.Saturday; day++ {
			profile.OrderTimingPreference[day] = []int{12, 19}
		}
	}

	return profile
}

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

	// calculate segment and order frequency
	segment := uf.assignUserSegment()
	orderFrequency := uf.calculateInitialOrderFrequency(segment, config)

	// generate user-behaviour profile
	profile := uf.createUserBehaviorProfile(segment)

	// generate preferences based on behaviour
	preferences := uf.generateWeightedPreferences(profile)

	return &models.User{
		ID:       cuid.New(),
		Name:     fake.Person().Name(),
		Email:    fake.Internet().Email(),
		JoinDate: fake.Time().TimeBetween(config.StartDate.AddDate(-1, 0, 0), config.StartDate),
		Location: models.Location{
			Lat: lat,
			Lon: lon,
		},
		Preferences:         preferences,
		DietaryRestrictions: generateRandomDietaryRestrictions(),
		OrderFrequency:      orderFrequency,
		Segment:             segment,
		PurchasePatterns:    make(map[time.Weekday][]int),
		LastOrderTime:       time.Time{},
		LifetimeOrders:      0,
		LifetimeSpend:       0.0,
	}
}

func (uf *UserFactory) generateWeightedPreferences(profile models.UserBehaviourProfile) []string {
	allCuisines := []string{
		"British",
		"Pub Food",
		"Fish and Chips",
		"Indian",
		"Bangladeshi",
		"Pakistani",
		"Chinese",
		"Thai",
		"Japanese",
		"Korean",
		"Vietnamese",
		"Malaysian",
		"Singaporean",
		"Indonesian",
		"Asian Fusion",
		"Italian",
		"Pizza",
		"Pasta",
		"French",
		"Spanish",
		"Tapas",
		"Greek",
		"Turkish",
		"Middle Eastern",
		"Lebanese",
		"Moroccan",
		"Persian",
		"African",
		"Ethiopian",
		"Caribbean",
		"Jamaican",
		"American",
		"Mexican",
		"Tex-Mex",
		"Brazilian",
		"Argentinian",
		"Peruvian",
		"Polish",
		"Russian",
		"German",
		"Scandinavian",
		"Vegan",
		"Vegetarian",
		"Gluten-Free",
		"Halal",
		"Kosher",
		"Steakhouse",
		"Seafood",
		"BBQ",
		"Burgers",
		"Sushi",
		"Noodles",
		"Dumplings",
		"Salads",
		"Sandwiches",
		"Fast Food",
		"Desserts",
		"Bakery",
		"Coffee and Tea",
		"Ice Cream",
		"Juice Bar",
		"Healthy",
		"Organic",
		"Fusion",
		"International",
		"Mediterranean",
		"Curry",
		"Kebabs",
		"Wraps",
		"Pancakes",
		"Waffles",
		"Crepes",
		"Fine Dining",
		"Casual Dining",
		"Buffet",
		"Street Food",
	}

	// generate weights based on profile
	weights := make([]float64, len(allCuisines))
	totalWeight := 0.0

	for i, cuisine := range allCuisines {
		// base weight
		weight := 1.0

		// adjust based on profile preferences
		if w, exists := profile.CuisineWeights[cuisine]; exists {
			weight *= w
		}

		weights[i] = weight
		totalWeight += weight
	}

	// select preferences probabilistically
	preferences := make([]string, 0)
	prefCount := rand.Intn(3) + 1 // 1 to 3 preferences

	for i := 0; i < prefCount; i++ {
		selected := uf.selectWeighted(allCuisines, weights, totalWeight)
		if selected != "" {
			preferences = append(preferences, selected)
		}
	}
	return preferences
}

func (uf *UserFactory) selectWeighted(items []string, weights []float64, totalWeight float64) string {
	if len(items) == 0 || totalWeight == 0 {
		return ""
	}

	r := rand.Float64() * totalWeight
	currentSum := 0.0

	for i, item := range items {
		currentSum += weights[i]
		if r <= currentSum {
			return item
		}
	}

	return items[len(items)-1]
}

func (uf *UserFactory) calculateInitialOrderFrequency(segment string, config *models.Config) float64 {
	baseFrequency := models.DefaultCustomerSegments[segment].OrdersPerMonth / 30.0

	// add some randomisation
	randomFactor := 0.8 + (rand.Float64() * 0.4) // ±20%

	return baseFrequency * randomFactor * config.OrderFrequency
}

func generateRandomDietaryRestrictions() []string {
	restrictions := []string{"Vegetarian", "Vegan", "Gluten-free", "Dairy-free", "Nut-free", "Halal", "Kosher"}
	restrictCount := fake.IntBetween(0, 2)
	if restrictCount == 0 {
		return nil
	}
	return []string{restrictions[rand.Intn(len(restrictions))]}
}
