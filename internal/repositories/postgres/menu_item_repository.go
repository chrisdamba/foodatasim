package postgres

import (
	"context"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type MenuItemRepository struct {
	pool *pgxpool.Pool
}

func NewMenuItemRepository(pool *pgxpool.Pool) *MenuItemRepository {
	return &MenuItemRepository{pool: pool}
}

func (r *MenuItemRepository) BulkCreate(ctx context.Context, menuItems []*models.MenuItem) error {
	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"menu_items"},
		[]string{
			"id", "restaurant_id", "name", "description", "price",
			"prep_time", "category", "type", "popularity",
			"prep_complexity", "ingredients", "is_discount_eligible",
		},
		pgx.CopyFromSlice(len(menuItems), func(i int) ([]interface{}, error) {
			return []interface{}{
				menuItems[i].ID,
				menuItems[i].RestaurantID,
				menuItems[i].Name,
				menuItems[i].Description,
				menuItems[i].Price,
				menuItems[i].PrepTime,
				menuItems[i].Category,
				menuItems[i].Type,
				menuItems[i].Popularity,
				menuItems[i].PrepComplexity,
				menuItems[i].Ingredients,
				menuItems[i].IsDiscountEligible,
			}, nil
		}),
	)
	return err
}

func (r *MenuItemRepository) Create(ctx context.Context, menuItem *models.MenuItem) error {
	query := `
        INSERT INTO menu_items (
            id, restaurant_id, name, description, price, prep_time,
            category, type, popularity, prep_complexity, ingredients,
            is_discount_eligible
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
        )
    `

	_, err := r.pool.Exec(ctx, query,
		menuItem.ID,
		menuItem.RestaurantID,
		menuItem.Name,
		menuItem.Description,
		menuItem.Price,
		menuItem.PrepTime,
		menuItem.Category,
		menuItem.Type,
		menuItem.Popularity,
		menuItem.PrepComplexity,
		menuItem.Ingredients,
		menuItem.IsDiscountEligible,
	)
	return err
}

func (r *MenuItemRepository) GetAll(ctx context.Context) (map[string]*models.MenuItem, error) {
	query := `
        SELECT 
            id, 
            restaurant_id, 
            name, 
            description, 
            price, 
            prep_time, 
            category, 
            type, 
            popularity, 
            prep_complexity, 
            ingredients, 
            is_discount_eligible,
            created_at,
            updated_at
        FROM menu_items
    `
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menuItems := make(map[string]*models.MenuItem)
	for rows.Next() {
		menuItem := &models.MenuItem{}
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&menuItem.ID,
			&menuItem.RestaurantID,
			&menuItem.Name,
			&menuItem.Description,
			&menuItem.Price,
			&menuItem.PrepTime,
			&menuItem.Category,
			&menuItem.Type,
			&menuItem.Popularity,
			&menuItem.PrepComplexity,
			&menuItem.Ingredients,
			&menuItem.IsDiscountEligible,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}
		menuItems[menuItem.ID] = menuItem
	}
	return menuItems, nil
}

func (r *MenuItemRepository) GetPopularItems(ctx context.Context, minPopularity float64) ([]*models.MenuItem, error) {
	query := `
        SELECT * FROM menu_items 
        WHERE popularity >= $1 
        ORDER BY popularity DESC
    `

	rows, err := r.pool.Query(ctx, query, minPopularity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var menuItems []*models.MenuItem
	for rows.Next() {
		menuItem := &models.MenuItem{}
		err := rows.Scan(
			&menuItem.ID,
			&menuItem.RestaurantID,
			&menuItem.Name,
			&menuItem.Description,
			&menuItem.Price,
			&menuItem.PrepTime,
			&menuItem.Category,
			&menuItem.Type,
			&menuItem.Popularity,
			&menuItem.PrepComplexity,
			&menuItem.Ingredients,
			&menuItem.IsDiscountEligible,
		)
		if err != nil {
			return nil, err
		}
		menuItems = append(menuItems, menuItem)
	}
	return menuItems, nil
}

func (r *MenuItemRepository) FindByIngredient(ctx context.Context, ingredient string) ([]*models.MenuItem, error) {
	query := `
        SELECT * FROM menu_items 
        WHERE $1 = ANY(ingredients)
    `

	rows, err := r.pool.Query(ctx, query, ingredient)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var menuItems []*models.MenuItem
	for rows.Next() {
		menuItem := &models.MenuItem{}
		err := rows.Scan(
			&menuItem.ID,
			&menuItem.RestaurantID,
			&menuItem.Name,
			&menuItem.Description,
			&menuItem.Price,
			&menuItem.PrepTime,
			&menuItem.Category,
			&menuItem.Type,
			&menuItem.Popularity,
			&menuItem.PrepComplexity,
			&menuItem.Ingredients,
			&menuItem.IsDiscountEligible,
		)
		if err != nil {
			return nil, err
		}
		menuItems = append(menuItems, menuItem)
	}
	return menuItems, nil
}

func (r *MenuItemRepository) GetByRestaurantID(ctx context.Context, restaurantID string) ([]*models.MenuItem, error) {
	query := `SELECT * FROM menu_items WHERE restaurant_id = $1`
	rows, err := r.pool.Query(ctx, query, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var menuItems []*models.MenuItem
	for rows.Next() {
		menuItem := &models.MenuItem{}
		err := rows.Scan(
			&menuItem.ID,
			&menuItem.RestaurantID,
			&menuItem.Name,
			&menuItem.Description,
			&menuItem.Price,
			&menuItem.PrepTime,
			&menuItem.Category,
			&menuItem.Type,
			&menuItem.Popularity,
			&menuItem.PrepComplexity,
			&menuItem.Ingredients,
			&menuItem.IsDiscountEligible,
		)
		if err != nil {
			return nil, err
		}
		menuItems = append(menuItems, menuItem)
	}
	return menuItems, nil
}

func (r *MenuItemRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM menu_items").Scan(&count)
	return count, err
}

func (r *MenuItemRepository) DeleteAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "TRUNCATE TABLE menu_items CASCADE")
	return err
}
