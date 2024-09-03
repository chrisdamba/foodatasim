package models

type Address struct {
	HouseNo   string  `json:"house_no"`
	Flat      string  `json:"flat"`
	Address1  string  `json:"address1"`
	Address2  string  `json:"address2"`
	Postcode  string  `json:"postcode"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
