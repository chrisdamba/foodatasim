package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lucsky/cuid"
	"math/rand"
	"strings"
)

type MenuItemFactory struct{}

func (mf *MenuItemFactory) CreateMenuItem(restaurant *models.Restaurant, config *models.Config) models.MenuItem {
	menuItemName := generateRandomMenuItem(restaurant.Cuisines, config)
	if len(menuItemName) > 255 {
		menuItemName = menuItemName[:252] + "..."
	}
	return models.MenuItem{
		ID:                 cuid.New(),
		RestaurantID:       restaurant.ID,
		Name:               sanitiseString(menuItemName),
		Description:        sanitiseString(fake.Lorem().Sentence(10)),
		Price:              fake.Float64(2, 5, 50),
		PrepTime:           fake.Float64(0, 5, 30),
		Category:           sanitiseString(fake.Lorem().Word()),
		Type:               generateRandomMenuItemType(),
		Popularity:         fake.Float64(2, 0, 100) / 100,
		PrepComplexity:     fake.Float64(2, 0, 100) / 100,
		Ingredients:        generateRandomIngredients(),
		IsDiscountEligible: fake.Bool(),
	}
}

func generateRandomIngredients() []string {
	allIngredients := []string{"Chicken", "Beef", "Pork", "Fish", "Tofu", "Cheese", "Tomato", "Lettuce", "Onion", "Garlic", "Bread", "Rice", "Pasta", "Egg", "Milk"}
	ingredientCount := rand.Intn(5) + 2 // 2 to 6 ingredients
	ingredients := make([]string, ingredientCount)
	for i := 0; i < ingredientCount; i++ {
		ingredients[i] = allIngredients[rand.Intn(len(allIngredients))]
	}
	return ingredients
}

func generateRandomMenuItem(cuisines []string, config *models.Config) string {
	// check if config has menu dishes
	if len(config.MenuDishes) > 0 {
		// randomly choose between 1 and 5 dishes from the config
		dishCount := rand.Intn(5) + 1
		menuDishes := make([]string, dishCount)
		for i := 0; i < dishCount; i++ {
			menuDishes[i] = config.MenuDishes[rand.Intn(len(config.MenuDishes))].Name
		}
		return strings.Join(menuDishes, ", ")
	}

	// fallback to the default predefined items based on cuisine
	items := map[string][]string{
		"Pizza":         {"Margherita", "Pepperoni", "Hawaiian", "Veggie Supreme"},
		"Curry":         {"Chicken Tikka Masala", "Vegetable Curry", "Beef Madras", "Paneer Butter Masala"},
		"Burgers":       {"Classic Cheeseburger", "Veggie Burger", "BBQ Bacon Burger", "Mushroom Swiss Burger"},
		"Grill":         {"Grilled Chicken", "BBQ Ribs", "Grilled Salmon", "Mixed Grill Platter"},
		"Salad":         {"Caesar Salad", "Greek Salad", "Cobb Salad", "Quinoa Salad"},
		"Milkshake":     {"Chocolate Shake", "Vanilla Shake", "Strawberry Shake", "Oreo Shake"},
		"Italian":       {"Margherita Pizza", "Spaghetti Carbonara", "Lasagna", "Tiramisu"},
		"Indian":        {"Chicken Tikka Masala", "Vegetable Curry", "Naan Bread", "Biryani"},
		"American":      {"Cheeseburger", "Hot Dog", "BBQ Ribs", "Apple Pie"},
		"Japanese":      {"Sushi Roll", "Ramen", "Tempura", "Miso Soup"},
		"Mexican":       {"Tacos", "Burrito", "Guacamole", "Quesadilla"},
		"Chinese":       {"Kung Pao Chicken", "Fried Rice", "Dumplings", "Mapo Tofu"},
		"Thai":          {"Pad Thai", "Green Curry", "Tom Yum Soup", "Mango Sticky Rice"},
		"Greek":         {"Gyros", "Greek Salad", "Moussaka", "Baklava"},
		"French":        {"Coq au Vin", "Beef Bourguignon", "Ratatouille", "Crème Brûlée"},
		"Mediterranean": {"Falafel", "Hummus", "Tabbouleh", "Grilled Halloumi"},
	}
	cuisine := cuisines[rand.Intn(len(cuisines))]
	if items, ok := items[cuisine]; ok {
		return items[rand.Intn(len(items))]
	}
	return "Special of the Day"
}

func generateRandomMenuItemType() string {
	types := []string{"appetizer", "main course", "side dish", "dessert", "drink"}
	return types[rand.Intn(len(types))]
}

func sanitiseString(s string) string {
	// remove any non-printable characters and ensure UTF-8 validity
	valid := make([]rune, 0, len(s))
	for _, r := range s {
		if r > 31 && r != 127 && r < 65533 { // filter out control characters and invalid Unicode
			valid = append(valid, r)
		}
	}
	// replace any remaining problematic characters with spaces
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 || r >= 127 {
			return ' '
		}
		return r
	}, string(valid))

	// remove multiple spaces
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	return cleaned
}
