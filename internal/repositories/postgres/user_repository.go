package postgres

import (
	"context"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) BulkCreate(ctx context.Context, users []*models.User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	stmt := `
        INSERT INTO users (
            id, name, email, join_date, location, preferences, 
            dietary_restrictions, order_frequency, segment, behavior_profile,
            purchase_patterns, order_history, lifetime_orders, lifetime_spend
        ) VALUES (
            $1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8, $9,
            $10, $11, $12, $13, $14, $15
        )`

	for _, user := range users {
		_, err = tx.Exec(ctx, stmt,
			user.ID,
			user.Name,
			user.Email,
			user.JoinDate,
			user.Location.Lon,
			user.Location.Lat,
			user.Preferences,
			user.DietaryRestrictions,
			user.OrderFrequency,
			user.Segment,
			user.BehaviorProfile,
			user.PurchasePatterns,
			user.OrderHistory,
			user.LifetimeOrders,
			user.LifetimeSpend,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (
            id, name, email, join_date, location, preferences,
            dietary_restrictions, order_frequency, segment, behavior_profile,
            purchase_patterns, order_history, lifetime_orders, lifetime_spend
        ) VALUES (
            $1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8, $9,
            $10, $11, $12, $13, $14, $15
        )
    `
	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Name,
		user.Email,
		user.JoinDate,
		user.Location.Lon,
		user.Location.Lat,
		user.Preferences,
		user.DietaryRestrictions,
		user.OrderFrequency,
		user.Segment,
		user.BehaviorProfile,
		user.PurchasePatterns,
		user.OrderHistory,
		user.LifetimeOrders,
		user.LifetimeSpend,
	)
	return err
}

func (r *UserRepository) GetAll(ctx context.Context) ([]*models.User, error) {
	query := `
        SELECT 
            id, name, email, join_date, ST_X(location::geometry) as longitude,
            ST_Y(location::geometry) as latitude, preferences, dietary_restrictions,
            order_frequency, segment, behavior_profile, purchase_patterns,
            order_history, lifetime_orders, lifetime_spend
        FROM users`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var lon, lat float64
		user := &models.User{}
		err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.JoinDate,
			&lon,
			&lat,
			&user.Preferences,
			&user.DietaryRestrictions,
			&user.OrderFrequency,
			&user.Segment,
			&user.BehaviorProfile,
			&user.PurchasePatterns,
			&user.OrderHistory,
			&user.LifetimeOrders,
			&user.LifetimeSpend,
		)
		if err != nil {
			return nil, err
		}
		user.Location = models.Location{Lon: lon, Lat: lat}
		users = append(users, user)
	}
	return users, nil
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *UserRepository) DeleteAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "TRUNCATE TABLE users CASCADE")
	return err
}
