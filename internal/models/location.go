package models

import "fmt"

type Location struct {
	Lat float64 `json:"lat" parquet:"name=lat,type=DOUBLE"`
	Lon float64 `json:"lon" parquet:"name=lon,type=DOUBLE"`
}

// Hotspot represents a location with high demand for food delivery
type Hotspot struct {
	Location Location
	Weight   float64 // Represents the importance or activity level of the hotspot
}

type PartnerLocationUpdate struct {
	PartnerID   string
	OrderID     string
	NewLocation Location
	Speed       float64
}

type LocationCluster struct {
	Name                string
	FoodRatingMean      float64
	DeliveryRatingMean  float64
	OrderDensity        float64
	PreparationTimeMean float64
	DeliveryTimeMean    float64
}

var DefaultLocationClusters = []LocationCluster{
	{
		Name:                "urban",
		FoodRatingMean:      4.5,
		DeliveryRatingMean:  4.3,
		OrderDensity:        1.2,
		PreparationTimeMean: 25,
		DeliveryTimeMean:    20,
	},
	{
		Name:                "suburban",
		FoodRatingMean:      4.2,
		DeliveryRatingMean:  4.0,
		OrderDensity:        1.0,
		PreparationTimeMean: 30,
		DeliveryTimeMean:    30,
	},
	{
		Name:                "rural",
		FoodRatingMean:      3.8,
		DeliveryRatingMean:  3.7,
		OrderDensity:        0.8,
		PreparationTimeMean: 35,
		DeliveryTimeMean:    45,
	},
}

func (l *Location) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []byte:
		_, err := fmt.Sscanf(string(v), "POINT(%f %f)", &l.Lon, &l.Lat)
		return err
	case string:
		_, err := fmt.Sscanf(v, "POINT(%f %f)", &l.Lon, &l.Lat)
		return err
	default:
		return fmt.Errorf("unsupported type for Location: %T", value)
	}
}
