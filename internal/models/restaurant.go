package models

type Restaurant struct {
	ID               string   `json:"id"`
	Host             string   `json:"host"`
	Name             string   `json:"name"`
	Currency         int      `json:"currency"`
	Phone            string   `json:"phone"`
	Town             string   `json:"town"`
	SlugName         string   `json:"slug_name"`
	WebsiteLogoURL   string   `json:"website_logo_url"`
	Offline          string   `json:"offline"`
	Location         Location `json:"location"`
	Cuisines         []string `json:"cuisines"`
	Rating           float64  `json:"rating"`
	AvgPrepTime      float64  `json:"avg_prep_time"`
	PickupEfficiency float64  `json:"pickup_efficiency"`
	MenuItems        []string `json:"menu_item_ids"`
}
