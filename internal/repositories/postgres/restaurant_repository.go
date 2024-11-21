package postgres

import (
	"context"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RestaurantRepository struct {
	pool *pgxpool.Pool
}

func NewRestaurantRepository(pool *pgxpool.Pool) *RestaurantRepository {
	return &RestaurantRepository{pool: pool}
}

func (r *RestaurantRepository) BulkCreate(ctx context.Context, restaurants []*models.Restaurant) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, restaurant := range restaurants {
		query := `
            INSERT INTO restaurants (
                id, host, name, currency, phone, town, slug_name, website_logo_url,
                offline, location, cuisines, rating, total_ratings, prep_time,
                min_prep_time, avg_prep_time, pickup_efficiency, capacity
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9,
                ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
                $12, $13, $14, $15, $16, $17, $18, $19
            )
        `

		_, err = tx.Exec(ctx, query,
			restaurant.ID,
			restaurant.Host,
			restaurant.Name,
			restaurant.Currency,
			restaurant.Phone,
			restaurant.Town,
			restaurant.SlugName,
			restaurant.WebsiteLogoURL,
			restaurant.Offline,
			restaurant.Location.Lon,
			restaurant.Location.Lat,
			restaurant.Cuisines,
			restaurant.Rating,
			restaurant.TotalRatings,
			restaurant.PrepTime,
			restaurant.MinPrepTime,
			restaurant.AvgPrepTime,
			restaurant.PickupEfficiency,
			restaurant.Capacity,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RestaurantRepository) GetAll(ctx context.Context) (map[string]*models.Restaurant, error) {
	// First get all restaurants
	query := `
        SELECT 
            id, host, name, currency, phone, town, slug_name, website_logo_url,
            offline, ST_X(location::geometry) as longitude, ST_Y(location::geometry) as latitude,
            cuisines, rating, total_ratings, prep_time, min_prep_time, avg_prep_time,
            pickup_efficiency, capacity
        FROM restaurants
    `
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	restaurants := make(map[string]*models.Restaurant)
	for rows.Next() {
		var lon, lat float64
		restaurant := &models.Restaurant{}
		err := rows.Scan(
			&restaurant.ID,
			&restaurant.Host,
			&restaurant.Name,
			&restaurant.Currency,
			&restaurant.Phone,
			&restaurant.Town,
			&restaurant.SlugName,
			&restaurant.WebsiteLogoURL,
			&restaurant.Offline,
			&lon,
			&lat,
			&restaurant.Cuisines,
			&restaurant.Rating,
			&restaurant.TotalRatings,
			&restaurant.PrepTime,
			&restaurant.MinPrepTime,
			&restaurant.AvgPrepTime,
			&restaurant.PickupEfficiency,
			&restaurant.Capacity,
		)
		if err != nil {
			return nil, err
		}
		restaurant.Location = models.Location{Lon: lon, Lat: lat}
		restaurants[restaurant.ID] = restaurant
	}

	// Then get menu items for all restaurants
	menuQuery := `SELECT * FROM menu_items`
	menuRows, err := r.pool.Query(ctx, menuQuery)
	if err != nil {
		return nil, err
	}
	defer menuRows.Close()

	for menuRows.Next() {
		menuItem := models.MenuItem{}
		err := menuRows.Scan(
			&menuItem.ID,
		)
		if err != nil {
			return nil, err
		}

		if restaurant, exists := restaurants[menuItem.RestaurantID]; exists {
			restaurant.MenuItems = append(restaurant.MenuItems, menuItem.ID)
		}
	}

	return restaurants, nil
}

func (r *RestaurantRepository) Create(ctx context.Context, restaurant *models.Restaurant) error {
	query := `
        INSERT INTO restaurants (
            id, host, name, currency, phone, town, slug_name, website_logo_url,
            offline, location, cuisines, rating, total_ratings, prep_time,
            min_prep_time, avg_prep_time, pickup_efficiency, capacity
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9,
            ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
            $12, $13, $14, $15, $16, $17, $18, $19
        )
    `

	_, err := r.pool.Exec(ctx, query,
		restaurant.ID,
		restaurant.Host,
		restaurant.Name,
		restaurant.Currency,
		restaurant.Phone,
		restaurant.Town,
		restaurant.SlugName,
		restaurant.WebsiteLogoURL,
		restaurant.Offline,
		restaurant.Location.Lon,
		restaurant.Location.Lat,
		restaurant.Cuisines,
		restaurant.Rating,
		restaurant.TotalRatings,
		restaurant.PrepTime,
		restaurant.MinPrepTime,
		restaurant.AvgPrepTime,
		restaurant.PickupEfficiency,
		restaurant.Capacity,
	)
	return err
}

func (r *RestaurantRepository) FindNearby(ctx context.Context, location models.Location, radiusMeters float64) ([]*models.Restaurant, error) {
	query := `
        SELECT 
            id, host, name, currency, phone, town, slug_name, website_logo_url,
            offline, ST_X(location::geometry) as longitude, ST_Y(location::geometry) as latitude,
            cuisines, rating, total_ratings, prep_time, min_prep_time, avg_prep_time,
            pickup_efficiency, capacity,
            ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance
        FROM restaurants
        WHERE ST_DWithin(
            location,
            ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
            $3
        )
        ORDER BY distance
    `

	rows, err := r.pool.Query(ctx, query, location.Lon, location.Lat, radiusMeters)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var restaurants []*models.Restaurant
	for rows.Next() {
		var lon, lat, distance float64
		restaurant := &models.Restaurant{}
		err := rows.Scan(
			&restaurant.ID,
			&restaurant.Host,
			&restaurant.Name,
			&restaurant.Currency,
			&restaurant.Phone,
			&restaurant.Town,
			&restaurant.SlugName,
			&restaurant.WebsiteLogoURL,
			&restaurant.Offline,
			&lon,
			&lat,
			&restaurant.Cuisines,
			&restaurant.Rating,
			&restaurant.TotalRatings,
			&restaurant.PrepTime,
			&restaurant.MinPrepTime,
			&restaurant.AvgPrepTime,
			&restaurant.PickupEfficiency,
			&restaurant.Capacity,
			&distance,
		)
		if err != nil {
			return nil, err
		}
		restaurant.Location = models.Location{Lon: lon, Lat: lat}
		restaurants = append(restaurants, restaurant)
	}
	return restaurants, nil
}

func (r *RestaurantRepository) FindByCuisine(ctx context.Context, cuisine string) ([]*models.Restaurant, error) {
	query := `
        SELECT 
            id, host, name, currency, phone, town, slug_name, website_logo_url,
            offline, ST_X(location::geometry) as longitude, ST_Y(location::geometry) as latitude,
            cuisines, rating, total_ratings, prep_time, min_prep_time, avg_prep_time,
            pickup_efficiency, capacity
        FROM restaurants
        WHERE $1 = ANY(cuisines)
    `

	rows, err := r.pool.Query(ctx, query, cuisine)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var restaurants []*models.Restaurant
	for rows.Next() {
		var lon, lat, distance float64
		restaurant := &models.Restaurant{}
		err := rows.Scan(
			&restaurant.ID,
			&restaurant.Host,
			&restaurant.Name,
			&restaurant.Currency,
			&restaurant.Phone,
			&restaurant.Town,
			&restaurant.SlugName,
			&restaurant.WebsiteLogoURL,
			&restaurant.Offline,
			&lon,
			&lat,
			&restaurant.Cuisines,
			&restaurant.Rating,
			&restaurant.TotalRatings,
			&restaurant.PrepTime,
			&restaurant.MinPrepTime,
			&restaurant.AvgPrepTime,
			&restaurant.PickupEfficiency,
			&restaurant.Capacity,
			&distance,
		)
		if err != nil {
			return nil, err
		}
		restaurant.Location = models.Location{Lon: lon, Lat: lat}
		restaurants = append(restaurants, restaurant)
	}
	return restaurants, nil
}

func (r *RestaurantRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM restaurants").Scan(&count)
	return count, err
}

func (r *RestaurantRepository) DeleteAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "TRUNCATE TABLE restaurants CASCADE")
	return err
}
