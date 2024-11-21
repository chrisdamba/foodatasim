package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math/rand"
)

type MenuItemFactory struct{}

func (mf *MenuItemFactory) CreateMenuItem(restaurant *models.Restaurant, config *models.Config) models.MenuItem {
	name := truncateString(generateRandomMenuItem(restaurant.Cuisines, config), 250) // Leave some buffer for the 255 limit
	description := truncateString(fake.Lorem().Sentence(10), 250)

	return models.MenuItem{
		ID:                 cuid.New(),
		RestaurantID:       restaurant.ID,
		Name:               name,
		Description:        description,
		Price:              fake.Float64(2, 5, 50),
		PrepTime:           fake.Float64(0, 5, 30),
		Category:           truncateString(fake.Lorem().Word(), 250),
		Type:               generateRandomMenuItemType(),
		Popularity:         fake.Float64(2, 0, 100) / 100,
		PrepComplexity:     fake.Float64(2, 0, 100) / 100,
		Ingredients:        generateRandomIngredients(),
		IsDiscountEligible: fake.Bool(),
	}
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength]
}

func generateRandomMenuItem(cuisines []string, config *models.Config) string {
	if len(config.MenuDishes) > 0 {
		return config.MenuDishes[rand.Intn(len(config.MenuDishes))].Name
	}

	items := map[string][]string{
		"Pizza":         {"Margherita", "Pepperoni", "Hawaiian", "Veggie"},
		"Curry":         {"Chicken Tikka", "Veg Curry", "Beef Curry", "Paneer"},
		"Burgers":       {"Classic Burger", "Veggie Burger", "BBQ Burger", "Swiss Burger"},
		"Grill":         {"Grilled Chicken", "BBQ Ribs", "Grilled Fish", "Mixed Grill"},
		"Salad":         {"Caesar", "Greek", "Cobb", "Quinoa"},
		"Milkshake":     {"Chocolate", "Vanilla", "Strawberry", "Oreo"},
		"Italian":       {"Margherita", "Carbonara", "Lasagna", "Tiramisu"},
		"Indian":        {"Butter Chicken", "Dal Makhani", "Naan", "Biryani"},
		"American":      {"Burger", "Hot Dog", "BBQ Ribs", "Apple Pie"},
		"Japanese":      {"Sushi", "Ramen", "Tempura", "Miso"},
		"Mexican":       {"Tacos", "Burrito", "Guacamole", "Quesadilla"},
		"Chinese":       {"Kung Pao", "Fried Rice", "Dumplings", "Mapo Tofu"},
		"Thai":          {"Pad Thai", "Green Curry", "Tom Yum", "Mango Rice"},
		"Greek":         {"Gyros", "Greek Salad", "Moussaka", "Baklava"},
		"French":        {"Coq au Vin", "Bourguignon", "Ratatouille", "Crème Brûlée"},
		"Mediterranean": {"Falafel", "Hummus", "Tabbouleh", "Halloumi"},
	}

	cuisine := cuisines[rand.Intn(len(cuisines))]
	if items, ok := items[cuisine]; ok {
		return items[rand.Intn(len(items))]
	}
	return "Daily Special"
}

func generateRandomIngredients() []string {
	allIngredients := []string{
		"Chicken", "Beef", "Pork", "Fish", "Tofu",
		"Cheese", "Tomato", "Lettuce", "Onion",
		"Garlic", "Bread", "Rice", "Pasta", "Egg", "Milk",
	}

	selectedIngredients := make(map[string]bool)
	ingredientCount := rand.Intn(5) + 2 // 2 to 6 ingredients

	result := make([]string, 0, ingredientCount)
	for len(result) < ingredientCount {
		ingredient := allIngredients[rand.Intn(len(allIngredients))]
		if !selectedIngredients[ingredient] {
			selectedIngredients[ingredient] = true
			result = append(result, ingredient)
		}
	}

	return result
}

func generateRandomMenuItemType() string {
	types := []string{"appetizer", "main course", "side dish", "dessert", "drink"}
	return types[rand.Intn(len(types))]
}
