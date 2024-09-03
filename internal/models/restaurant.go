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
	TotalRatings     float64  `json:"total_ratings"`
	PrepTime         float64  `json:"prep_time"`
	MinPrepTime      float64  `json:"min_prep_time"`
	AvgPrepTime      float64  `json:"avg_prep_time"` // Average preparation time in minutes
	PickupEfficiency float64  `json:"pickup_efficiency"`
	MenuItems        []string `json:"menu_item_ids"`
	CurrentOrders    []Order  `json:"current_orders"`
	Capacity         int      `json:"capacity"`
}
