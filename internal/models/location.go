package models

type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Hotspot represents a location with high demand for food delivery
type Hotspot struct {
	Location Location
	Weight   float64 // Represents the importance or activity level of the hotspot
}
