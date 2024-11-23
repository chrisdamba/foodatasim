package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math"
	"math/rand"
)

type DeliveryPartnerFactory struct{}

func (df *DeliveryPartnerFactory) CreateDeliveryPartner(config *models.Config) *models.DeliveryPartner {
	// use smaller radius for initial distribution
	latRange := (config.UrbanRadius * 0.7) / 111.0 // 70% of urban radius, converted to degrees
	lonRange := latRange / math.Cos(config.CityLat*math.Pi/180.0)

	// concentrate more partners near city center
	var latOffset, lonOffset float64
	if rand.Float64() < 0.7 { // 70% closer to center
		latOffset = (rand.Float64() * 0.5) * latRange * randSign()
		lonOffset = (rand.Float64() * 0.5) * lonRange * randSign()
	} else { // 30% in wider area
		latOffset = rand.Float64() * latRange * randSign()
		lonOffset = rand.Float64() * lonRange * randSign()
	}

	lat := config.CityLat + latOffset
	lon := config.CityLon + lonOffset

	return &models.DeliveryPartner{
		ID:           cuid.New(),
		Name:         fake.Person().Name(),
		JoinDate:     fake.Time().TimeBetween(config.StartDate.AddDate(-1, 0, 0), config.StartDate),
		Rating:       fake.Float64(1, 1, 5),
		TotalRatings: fake.Float64(0, 0, 500),
		Experience:   fake.Float64(2, 0, 100) / 100,
		AvgSpeed:     fake.Float64(1, 20, 60),
		Speed:        fake.Float64(1, 20, 60),
		CurrentLocation: models.Location{
			Lat: lat,
			Lon: lon,
		},
		Status:         models.PartnerStatusAvailable,
		LastUpdateTime: config.StartDate,
	}
}

func randSign() float64 {
	if rand.Float64() < 0.5 {
		return -1.0
	}
	return 1.0
}
