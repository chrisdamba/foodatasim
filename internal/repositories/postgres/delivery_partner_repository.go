package postgres

import (
	"context"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type DeliveryPartnerRepository struct {
	pool *pgxpool.Pool
}

func NewDeliveryPartnerRepository(pool *pgxpool.Pool) *DeliveryPartnerRepository {
	return &DeliveryPartnerRepository{pool: pool}
}

func (r *DeliveryPartnerRepository) BulkCreate(ctx context.Context, partners []*models.DeliveryPartner) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
        INSERT INTO delivery_partners (
            id, name, join_date, rating, total_ratings, experience,
            speed, avg_speed, current_order_id, current_location, 
            status, last_update_time
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, 
            ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
            $12::partner_status, $13
        )
    `

	for _, partner := range partners {
		_, err = tx.Exec(ctx, query,
			partner.ID,
			partner.Name,
			partner.JoinDate,
			partner.Rating,
			partner.TotalRatings,
			partner.Experience,
			partner.Speed,
			partner.AvgSpeed,
			partner.CurrentOrderID,
			partner.CurrentLocation.Lon,
			partner.CurrentLocation.Lat,
			partner.Status,
			partner.LastUpdateTime,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *DeliveryPartnerRepository) Create(ctx context.Context, partner *models.DeliveryPartner) error {
	query := `
        INSERT INTO delivery_partners (
            id, name, join_date, rating, total_ratings, experience,
            speed, avg_speed, current_order_id, current_location, 
            status, last_update_time
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, 
            ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
            $12::partner_status, $13
        )
    `

	_, err := r.pool.Exec(ctx, query,
		partner.ID,
		partner.Name,
		partner.JoinDate,
		partner.Rating,
		partner.TotalRatings,
		partner.Experience,
		partner.Speed,
		partner.AvgSpeed,
		partner.CurrentOrderID,
		partner.CurrentLocation.Lon,
		partner.CurrentLocation.Lat,
		partner.Status,
		partner.LastUpdateTime,
	)
	return err
}

func (r *DeliveryPartnerRepository) GetAll(ctx context.Context) ([]*models.DeliveryPartner, error) {
	query := `
        SELECT 
            id, name, join_date, rating, total_ratings, experience,
            speed, avg_speed, current_order_id,
            ST_X(current_location::geometry) as longitude,
            ST_Y(current_location::geometry) as latitude,
            status, last_update_time
        FROM delivery_partners
    `
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*models.DeliveryPartner
	for rows.Next() {
		var lon, lat float64
		partner := &models.DeliveryPartner{}
		err := rows.Scan(
			&partner.ID,
			&partner.Name,
			&partner.JoinDate,
			&partner.Rating,
			&partner.TotalRatings,
			&partner.Experience,
			&partner.Speed,
			&partner.AvgSpeed,
			&partner.CurrentOrderID,
			&lon,
			&lat,
			&partner.Status,
			&partner.LastUpdateTime,
		)
		if err != nil {
			return nil, err
		}
		partner.CurrentLocation = models.Location{Lon: lon, Lat: lat}
		partners = append(partners, partner)
	}
	return partners, nil
}

func (r *DeliveryPartnerRepository) FindAvailableNearby(ctx context.Context, location models.Location, radiusMeters float64) ([]*models.DeliveryPartner, error) {
	query := `
        SELECT 
            id, name, join_date, rating, total_ratings, experience,
            speed, avg_speed, current_order_id,
            ST_X(current_location::geometry) as longitude,
            ST_Y(current_location::geometry) as latitude,
            status, last_update_time,
            ST_Distance(current_location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance
        FROM delivery_partners
        WHERE 
            status = 'available'::partner_status
            AND ST_DWithin(
                current_location,
                ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
                $3
            )
        ORDER BY 
            rating DESC,
            distance ASC
    `

	rows, err := r.pool.Query(ctx, query, location.Lon, location.Lat, radiusMeters)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*models.DeliveryPartner
	for rows.Next() {
		var lon, lat, distance float64
		partner := &models.DeliveryPartner{}
		err := rows.Scan(
			&partner.ID,
			&partner.Name,
			&partner.JoinDate,
			&partner.Rating,
			&partner.TotalRatings,
			&partner.Experience,
			&partner.Speed,
			&partner.AvgSpeed,
			&partner.CurrentOrderID,
			&lon,
			&lat,
			&partner.Status,
			&partner.LastUpdateTime,
			&distance,
		)
		if err != nil {
			return nil, err
		}
		partner.CurrentLocation = models.Location{Lon: lon, Lat: lat}
		partners = append(partners, partner)
	}
	return partners, nil
}

func (r *DeliveryPartnerRepository) UpdateLocation(ctx context.Context, partnerID string, location models.Location) error {
	query := `
        UPDATE delivery_partners 
        SET 
            current_location = ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography,
            last_update_time = $4,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `

	_, err := r.pool.Exec(ctx, query,
		partnerID,
		location.Lon,
		location.Lat,
		time.Now(),
	)
	return err
}

func (r *DeliveryPartnerRepository) UpdateStatus(ctx context.Context, partnerID, orderID string, status string) error {
	query := `
        UPDATE delivery_partners 
        SET 
            status = $2::partner_status,
            current_order_id = $3,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `

	_, err := r.pool.Exec(ctx, query, partnerID, status, orderID)
	return err
}

func (r *DeliveryPartnerRepository) UpdateRating(ctx context.Context, partnerID string, newRating float64) error {
	query := `
        UPDATE delivery_partners 
        SET 
            rating = (rating * total_ratings + $2) / (total_ratings + 1),
            total_ratings = total_ratings + 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `

	_, err := r.pool.Exec(ctx, query, partnerID, newRating)
	return err
}

func (r *DeliveryPartnerRepository) GetByStatus(ctx context.Context, status string) ([]*models.DeliveryPartner, error) {
	query := `
        SELECT 
            id, name, join_date, rating, total_ratings, experience,
            speed, avg_speed, current_order_id,
            ST_X(current_location::geometry) as longitude,
            ST_Y(current_location::geometry) as latitude,
            status, last_update_time
        FROM delivery_partners
        WHERE status = $1::partner_status
    `

	rows, err := r.pool.Query(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []*models.DeliveryPartner
	for rows.Next() {
		var lon, lat float64
		partner := &models.DeliveryPartner{}
		err := rows.Scan(
			&partner.ID,
			&partner.Name,
			&partner.JoinDate,
			&partner.Rating,
			&partner.TotalRatings,
			&partner.Experience,
			&partner.Speed,
			&partner.AvgSpeed,
			&partner.CurrentOrderID,
			&lon,
			&lat,
			&partner.Status,
			&partner.LastUpdateTime,
		)
		if err != nil {
			return nil, err
		}
		partner.CurrentLocation = models.Location{Lon: lon, Lat: lat}
		partners = append(partners, partner)
	}
	return partners, nil
}

func (r *DeliveryPartnerRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM delivery_partners").Scan(&count)
	return count, err
}

func (r *DeliveryPartnerRepository) DeleteAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "TRUNCATE TABLE delivery_partners CASCADE")
	return err
}
