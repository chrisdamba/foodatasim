package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math"
)

type DeliveryPartnerFactory struct{}

func (df *DeliveryPartnerFactory) CreateDeliveryPartner(config *models.Config) *models.DeliveryPartner {
	// Calculate city bounds
	latRange := config.UrbanRadius / 111.0 // Approx. conversion from km to degrees
	lonRange := latRange / math.Cos(config.CityLat*math.Pi/180.0)

	lat := fake.Float64(6, int(config.CityLat-latRange), int(config.CityLat+latRange))
	lon := fake.Float64(6, int(config.CityLon-lonRange), int(config.CityLon+lonRange))

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
		Status: "available",
	}
}
