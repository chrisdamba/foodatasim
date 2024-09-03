package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
)

type DeliveryPartnerFactory struct{}

func (df *DeliveryPartnerFactory) CreateDeliveryPartner(config models.Config) models.DeliveryPartner {
	return models.DeliveryPartner{
		ID:           cuid.New(),
		Name:         fake.Person().Name(),
		JoinDate:     fake.Time().TimeBetween(config.StartDate.AddDate(-1, 0, 0), config.StartDate),
		Rating:       fake.Float64(1, 1, 5),
		TotalRatings: fake.Float64(0, 0, 500),
		Experience:   fake.Float64(2, 0, 100) / 100,
		AvgSpeed:     fake.Float64(1, 20, 60),
		CurrentLocation: models.Location{
			Lat: fake.Float64(6, -90, 90),
			Lon: fake.Float64(6, -180, 180),
		},
		Status: "available",
	}
}
