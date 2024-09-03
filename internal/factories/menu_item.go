package factories

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"math/rand"
)

type MenuItemFactory struct{}

func (mf *MenuItemFactory) CreateMenuItem(restaurant *models.Restaurant) models.MenuItem {
	return models.MenuItem{
		ID:                 fake.UUID().V4(),
		RestaurantID:       restaurant.ID,
		Name:               generateRandomMenuItem(restaurant.Cuisines),
		Description:        fake.Lorem().Sentence(10),
		Price:              fake.Float64(2, 5, 50),
		PrepTime:           fake.Float64(0, 5, 30),
		Category:           fake.Lorem().Word(),
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

func generateRandomMenuItem(cuisines []string) string {
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
