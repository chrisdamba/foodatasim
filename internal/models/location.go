package models

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
