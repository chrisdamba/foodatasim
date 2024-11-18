package output

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lib/pq"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	_ "github.com/lib/pq"
)

type PostgresOutput struct {
	db *sql.DB
}

func NewPostgresOutput(config *models.DatabaseConfig) (*PostgresOutput, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	return &PostgresOutput{db: db}, nil
}

func (p *PostgresOutput) WriteMessage(topic string, msg []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(msg, &event); err != nil {
		return err
	}

	table := topicToTable(topic)

	if table == "order_event" {
		if _, ok := event["delivery_address"]; ok {
			if deliveryAddr, ok := event["delivery_address"].(map[string]interface{}); ok {
				addressJSON, err := json.Marshal(deliveryAddr)
				if err != nil {
					return fmt.Errorf("failed to marshal delivery address: %w", err)
				}
				event["delivery_address"] = string(addressJSON)
			} else {
				event["delivery_address"] = "{}"
			}
		}
		// remove id if it exists since fact_order uses event_id BIGSERIAL
		delete(event, "id")
	} else if table == "orders" {
		delete(event, "event_type")
	} else {
		delete(event, "id")
	}

	if timestamp, ok := event["timestamp"].(float64); ok {
		event["timestamp"] = time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
	}

	if topic == "restaurant_event" {
		if event["current_capacity"] == nil {
			event["current_capacity"] = event["capacity"]
		}
		if event["orders_in_queue"] == nil {
			event["orders_in_queue"] = 0
		}
		if event["efficiency_score"] == nil && event["pickup_efficiency"] != nil {
			event["efficiency_score"] = event["pickup_efficiency"]
		}
	}

	if topic == "delivery_partner_event" {
		if ts, ok := event["timestamp"].(float64); ok {
			event["timestamp"] = time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05")
		}
		if updateTime, ok := event["update_time"].(float64); ok {
			event["update_time"] = time.Unix(int64(updateTime), 0).Format("2006-01-02 15:04:05")
		}
		if loc, ok := event["new_location"].(map[string]interface{}); ok {
			if lat, latOk := loc["lat"].(float64); latOk {
				if lon, lonOk := loc["lon"].(float64); lonOk {
					event["new_location"] = fmt.Sprintf("SRID=4326;POINT(%f %f)", lon, lat)
				}
			}
		}
		if loc, ok := event["current_location"].(map[string]interface{}); ok {
			if lat, latOk := loc["lat"].(float64); latOk {
				if lon, lonOk := loc["lon"].(float64); lonOk {
					event["current_location"] = fmt.Sprintf("SRID=4326;POINT(%f %f)", lon, lat)
				}
			}
		}
	}

	cols, vals, placeholders := buildInsertComponents(event)

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		cols,
		placeholders,
	)

	_, err := p.db.Exec(query, vals...)
	if err != nil {
		// log the query and values
		log.Printf("DEBUG: Executed query: %s", query)
		log.Printf("DEBUG: With values: %+v", vals)
		return fmt.Errorf("failed to insert into %s: %w", table, err)
	}

	return nil
}

func (p *PostgresOutput) BatchInsertUsers(users []*models.User) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("users",
		"id", "name", "join_date", "location",
		"preferences", "dietary_restrictions",
		"order_frequency"))
	if err != nil {
		return err
	}

	for _, user := range users {
		point := fmt.Sprintf("POINT(%f %f)", user.Location.Lon, user.Location.Lat)
		_, err = stmt.Exec(
			user.ID,
			user.Name,
			user.JoinDate,
			point,
			pq.Array(user.Preferences),
			pq.Array(user.DietaryRestrictions),
			user.OrderFrequency,
		)
		if err != nil {
			return err
		}
	}

	if err = stmt.Close(); err != nil {
		return err
	}

	return tx.Commit()
}

func (p *PostgresOutput) BatchInsertRestaurants(restaurants []*models.Restaurant) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Use a regular parameterized SQL INSERT
	insertSQL := `
        INSERT INTO restaurants (
            id, host, name, currency, phone,
            town, slug_name, website_logo_url, offline,
            location, cuisines, rating, total_ratings,
            prep_time, min_prep_time, avg_prep_time,
            pickup_efficiency, capacity, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9::offline_status,
            ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
            $12, $13, $14,
            $15, $16, $17,
            $18, $19, $20, $21
        )
    `

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()

	for i, restaurant := range restaurants {
		offlineStatus := "DISABLED"
		if restaurant.Offline == "true" {
			offlineStatus = "ENABLED"
		}

		_, err = stmt.Exec(
			restaurant.ID,
			restaurant.Host,
			restaurant.Name,
			restaurant.Currency,
			restaurant.Phone,
			restaurant.Town,
			restaurant.SlugName,
			restaurant.WebsiteLogoURL,
			offlineStatus,
			restaurant.Location.Lon, // longitude
			restaurant.Location.Lat, // latitude
			pq.Array(restaurant.Cuisines),
			restaurant.Rating,
			restaurant.TotalRatings,
			restaurant.PrepTime,
			restaurant.MinPrepTime,
			restaurant.AvgPrepTime,
			restaurant.PickupEfficiency,
			restaurant.Capacity,
			now,
			now,
		)
		if err != nil {
			log.Printf("Error inserting restaurant %d (ID: %s):", i, restaurant.ID)
			log.Printf("Restaurant data: %+v", restaurant)
			if pqErr, ok := err.(*pq.Error); ok {
				log.Printf("PostgreSQL Error: %s", pqErr.Message)
				log.Printf("PostgreSQL Detail: %s", pqErr.Detail)
				log.Printf("PostgreSQL Hint: %s", pqErr.Hint)
				log.Printf("PostgreSQL Code: %s", pqErr.Code)
			}
			return fmt.Errorf("failed to insert restaurant %s: %w", restaurant.ID, err)
		}
	}

	return tx.Commit()
}

func (p *PostgresOutput) BatchInsertDeliveryPartners(partners []*models.DeliveryPartner) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn(
		"delivery_partners",
		"id",
		"name",
		"join_date",
		"rating",
		"total_ratings",
		"experience",
		"speed",
		"avg_speed",
		"current_order_id",
		"current_location",
		"status",
		"last_update_time",
	))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, partner := range partners {
		// convert location to PostGIS format
		point := fmt.Sprintf("POINT(%f %f)",
			partner.CurrentLocation.Lon,
			partner.CurrentLocation.Lat)

		_, err = stmt.Exec(
			partner.ID,
			partner.Name,
			partner.JoinDate,
			partner.Rating,
			partner.TotalRatings,
			partner.Experience,
			partner.Speed,
			partner.AvgSpeed,
			nullableString(partner.CurrentOrderID), // Handle nullable foreign key
			point,
			partner.Status,
			partner.LastUpdateTime,
		)
		if err != nil {
			return fmt.Errorf("failed to exec statement for partner %s: %w", partner.ID, err)
		}
	}

	err = stmt.Close()
	if err != nil {
		return fmt.Errorf("failed to close statement: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresOutput) BatchUpdateDeliveryPartnerLocations(updates []models.PartnerLocationUpdate) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        UPDATE delivery_partners
        SET 
            current_location = ST_SetSRID(ST_MakePoint($2, $3), 4326),
            last_update_time = $4,
            speed = $5
        WHERE id = $1
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, update := range updates {
		_, err = stmt.Exec(
			update.PartnerID,
			update.NewLocation.Lon,
			update.NewLocation.Lat,
			time.Now(),
			update.Speed,
		)
		if err != nil {
			return fmt.Errorf("failed to update location for partner %s: %w", update.PartnerID, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresOutput) GetNearbyDeliveryPartners(loc models.Location, radius float64) ([]*models.DeliveryPartner, error) {
	query := `
        SELECT 
            id, name, join_date, rating, total_ratings,
            experience, speed, avg_speed, current_order_id,
            ST_X(current_location::geometry) as longitude,
            ST_Y(current_location::geometry) as latitude,
            status, last_update_time
        FROM delivery_partners
        WHERE 
            status = 'available'
            AND ST_DWithin(
                current_location,
                ST_SetSRID(ST_MakePoint($1, $2), 4326),
                $3 * 1000  -- Convert km to meters
            )
    `

	rows, err := p.db.Query(query, loc.Lon, loc.Lat, radius)
	if err != nil {
		return nil, fmt.Errorf("failed to query nearby partners: %w", err)
	}
	defer rows.Close()

	var partners []*models.DeliveryPartner
	for rows.Next() {
		var p models.DeliveryPartner
		var lon, lat float64
		var currentOrderID sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.JoinDate,
			&p.Rating,
			&p.TotalRatings,
			&p.Experience,
			&p.Speed,
			&p.AvgSpeed,
			&currentOrderID,
			&lon,
			&lat,
			&p.Status,
			&p.LastUpdateTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan partner row: %w", err)
		}

		p.CurrentLocation = models.Location{Lon: lon, Lat: lat}
		if currentOrderID.Valid {
			p.CurrentOrderID = currentOrderID.String
		}

		partners = append(partners, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating partner rows: %w", err)
	}

	return partners, nil
}

func (p *PostgresOutput) BatchInsertMenuItems(menuItems []*models.MenuItem) error {
	tx, err := p.db.Begin()
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, item := range menuItems {
		_, err = tx.Exec(`
            INSERT INTO menu_items (
                id, restaurant_id, name, description, price,
                prep_time, category, type, popularity,
                prep_complexity, ingredients, is_discount_eligible
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::menu_item_type, $9, $10, $11, $12)
        `,
			item.ID,
			item.RestaurantID,
			item.Name,
			item.Description,
			item.Price,
			item.PrepTime,
			item.Category,
			item.Type,
			item.Popularity,
			item.PrepComplexity,
			pq.Array(item.Ingredients),
			item.IsDiscountEligible,
		)
		if err != nil {
			log.Printf("Error inserting menu item %s: %v", item.ID, err)
			log.Printf("Menu item data: %+v", item)
			return fmt.Errorf("failed to insert menu item: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresOutput) BatchInsertOrdersTx(tx *sql.Tx, orders []*models.Order) error {
	stmt, err := tx.Prepare(pq.CopyIn(
		"orders", // Changed from "fact_order" to "orders"
		"id",
		"customer_id",
		"restaurant_id",
		"delivery_partner_id",
		"total_amount",
		"delivery_cost",
		"order_placed_at",
		"prep_start_time",
		"estimated_pickup_time",
		"estimated_delivery_time",
		"pickup_time",
		"in_transit_time",
		"actual_delivery_time",
		"status",
		"payment_method",
		"delivery_address",
		"review_generated",
	))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, order := range orders {
		// Convert address to JSON
		addressJSON, err := json.Marshal(map[string]interface{}{
			"lat": order.Address.Latitude,
			"lon": order.Address.Longitude,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal address: %w", err)
		}

		_, err = stmt.Exec(
			order.ID,
			order.CustomerID,
			order.RestaurantID,
			nullableString(order.DeliveryPartnerID),
			order.TotalAmount,
			order.DeliveryCost,
			order.OrderPlacedAt,
			nullableTime(order.PrepStartTime),
			nullableTime(order.EstimatedPickupTime),
			nullableTime(order.EstimatedDeliveryTime),
			nullableTime(order.PickupTime),
			nullableTime(order.InTransitTime),
			nullableTime(order.ActualDeliveryTime),
			order.Status,
			order.PaymentMethod,
			string(addressJSON),
			order.ReviewGenerated,
		)
		if err != nil {
			return fmt.Errorf("failed to exec statement for order %s: %w", order.ID, err)
		}
	}

	err = stmt.Close()
	if err != nil {
		return fmt.Errorf("failed to close statement: %w", err)
	}

	return nil
}

func (p *PostgresOutput) UpdateRestaurantOrderCountsTx(tx *sql.Tx, orders []*models.Order) error {
	// group orders by restaurant
	restaurantOrders := make(map[string]int)
	for _, order := range orders {
		restaurantOrders[order.RestaurantID]++
	}

	// update counts and metrics for each restaurant
	for restaurantID, count := range restaurantOrders {

		var currentRating float64
		var currentTotalRatings float64
		err := tx.QueryRow(`
            SELECT rating, total_ratings 
            FROM restaurants 
            WHERE id = $1
        `, restaurantID).Scan(&currentRating, &currentTotalRatings)
		if err != nil {
			return fmt.Errorf("failed to get restaurant data: %w", err)
		}

		// update the restaurant metrics
		_, err = tx.Exec(`
            UPDATE restaurants 
            SET total_ratings = total_ratings + $1,
                updated_at = NOW()
            WHERE id = $2
        `, count, restaurantID)
		if err != nil {
			return fmt.Errorf("failed to update restaurant metrics: %w", err)
		}
	}

	return nil
}

func (p *PostgresOutput) UpdateUserOrderCountsTx(tx *sql.Tx, orders []*models.Order) error {
	// Group orders by user
	userOrders := make(map[string]int)
	for _, order := range orders {
		userOrders[order.CustomerID]++
	}

	// Update counts for each user
	for userID, count := range userOrders {
		_, err := tx.Exec(`
            UPDATE users 
            SET total_orders = total_orders + $1,
                updated_at = NOW()
            WHERE id = $2
        `, count, userID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PostgresOutput) updateRestaurantMenuCounts(tx *sql.Tx) error {
	_, err := tx.Exec(`
        UPDATE restaurants r
        SET menu_item_count = (
            SELECT COUNT(*)
            FROM menu_items m
            WHERE m.restaurant_id = r.id
        )
    `)
	return err
}

func (p *PostgresOutput) GetMenuItemsByRestaurant(restaurantID string) ([]*models.MenuItem, error) {
	query := `
        SELECT 
            id, restaurant_id, name, description, price,
            prep_time, category, item_type, popularity,
            prep_complexity, ingredients, is_discount_eligible
        FROM menu_items
        WHERE restaurant_id = $1
        ORDER BY category, name
    `

	rows, err := p.db.Query(query, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query menu items: %w", err)
	}
	defer rows.Close()

	var items []*models.MenuItem
	for rows.Next() {
		var item models.MenuItem
		var description sql.NullString
		var ingredients pq.StringArray

		err := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.Name,
			&description,
			&item.Price,
			&item.PrepTime,
			&item.Category,
			&item.Type,
			&item.Popularity,
			&item.PrepComplexity,
			&ingredients,
			&item.IsDiscountEligible,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan menu item row: %w", err)
		}

		if description.Valid {
			item.Description = description.String
		}
		item.Ingredients = []string(ingredients)

		items = append(items, &item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating menu item rows: %w", err)
	}

	return items, nil
}

func (p *PostgresOutput) BatchUpdateMenuItemPrices(updates map[string]float64) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        UPDATE menu_items
        SET 
            price = $2,
            updated_at = NOW()
        WHERE id = $1
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for itemID, newPrice := range updates {
		_, err = stmt.Exec(itemID, newPrice)
		if err != nil {
			return fmt.Errorf("failed to update price for item %s: %w", itemID, err)
		}
	}

	return tx.Commit()
}

func (p *PostgresOutput) GetPopularMenuItems(limit int) ([]*models.MenuItem, error) {
	query := `
        SELECT 
            id, restaurant_id, name, description, price,
            prep_time, category, item_type, popularity,
            prep_complexity, ingredients, is_discount_eligible
        FROM menu_items
        WHERE popularity > 0.7  -- Threshold for "popular" items
        ORDER BY popularity DESC
        LIMIT $1
    `

	rows, err := p.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query popular items: %w", err)
	}
	defer rows.Close()

	var items []*models.MenuItem
	for rows.Next() {
		var item models.MenuItem
		var description sql.NullString
		var ingredients pq.StringArray

		err := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.Name,
			&description,
			&item.Price,
			&item.PrepTime,
			&item.Category,
			&item.Type,
			&item.Popularity,
			&item.PrepComplexity,
			&ingredients,
			&item.IsDiscountEligible,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan menu item row: %w", err)
		}

		if description.Valid {
			item.Description = description.String
		}
		item.Ingredients = []string(ingredients)

		items = append(items, &item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating menu item rows: %w", err)
	}

	return items, nil
}

func (p *PostgresOutput) Close() error {
	return p.db.Close()
}

func (p *PostgresOutput) BeginTx() (*sql.Tx, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

func (p *PostgresOutput) ExecTx(fn func(*sql.Tx) error) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresOutput) ExecTxWithRetry(fn func(*sql.Tx) error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = p.ExecTx(fn)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if isRetryableError(err) {
			time.Sleep(time.Duration(i*100) * time.Millisecond) // exponential backoff
			continue
		}

		return err // non-retryable error
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

func (p *PostgresOutput) BeginTxContext(ctx context.Context) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, nil)
}

func (p *PostgresOutput) ExecTxContext(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func nullableTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{
		Time:  t,
		Valid: true,
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific PostgreSQL error codes that indicate retryable errors
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "40001": // serialization_failure
		case "40P01": // deadlock_detected
		case "55P03": // lock_not_available
			return true
		}
	}

	return false
}

func topicToTable(topic string) string {
	tableMap := map[string]string{
		// order related events
		"order_placed_events":       "orders",
		"order_preparation_events":  "order_event",
		"order_ready_events":        "order_event",
		"order_pickup_events":       "order_event",
		"order_delivery_events":     "order_event",
		"order_cancellation_events": "order_event",
		"order_in_transit_events":   "order_event",

		// delivery performance events
		"delivery_status_check_events":       "delivery_partner_event",
		"partner_location_events":            "delivery_partner_event",
		"delivery_partner_events":            "delivery_partner_event",
		"delivery_partner_status_events":     "delivery_partner_event",
		"delivery_partner_shift_events":      "delivery_partner_event",
		"delivery_partner_assignment_events": "delivery_partner_event",

		// restaurant performance events
		"restaurant_status_events":   "restaurant_event",
		"restaurant_metrics_events":  "restaurant_event",
		"restaurant_menu_events":     "restaurant_event",
		"restaurant_capacity_events": "restaurant_event",
		"restaurant_hours_events":    "restaurant_event",

		// user/customer events
		"user_behaviour_events":  "customer_event",
		"user_preference_events": "customer_event",

		// review events
		"review_events": "review_event",

		//// time and location based events
		//"traffic_condition_events": "fact_traffic_condition",
		//"weather_condition_events": "fact_weather_condition",
		//"peak_hour_events":         "fact_peak_hours",
		//
		//// menu related facts
		//"menu_item_events":         "fact_menu_changes",
		//"menu_price_events":        "fact_menu_price",
		//"menu_availability_events": "fact_menu_availability",
		//
		//// promotion and discount facts
		//"promotion_events": "fact_promotion",
		//"discount_events":  "fact_discount",
		//
		//// payment facts
		//"payment_events": "fact_payment",
		//"refund_events":  "fact_refund",
		//
		//// service metrics facts
		//"service_quality_events":       "fact_service_quality",
		//"delivery_time_events":         "fact_delivery_time",
		//"customer_satisfaction_events": "fact_customer_satisfaction",
		//
		//// financial facts
		//"revenue_events":    "fact_revenue",
		//"cost_events":       "fact_cost",
		//"commission_events": "fact_commission",
		//
		//// operational facts
		//"capacity_utilization_events": "fact_capacity_utilization",
		//"efficiency_metrics_events":   "fact_efficiency_metrics",
		//"performance_metrics_events":  "fact_performance_metrics",
		//
		//// system facts
		//"notification_events":  "fact_notification",
		//"communication_events": "fact_communication",
		//"system_events":        "fact_system_log",
	}

	if table, ok := tableMap[topic]; ok {
		return table
	}
	// if no mapping found, use the topic name as table name
	// after converting from snake_case and removing _events suffix
	tableName := strings.TrimSuffix(topic, "_events")
	return "fact_" + tableName
}

func buildInsertComponents(event map[string]interface{}) (string, []interface{}, string) {
	var columns []string
	var values []interface{}
	var placeholderNum int
	var placeholders []string

	keys := make([]string, 0, len(event))
	for k := range event {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := event[key]

		switch v := val.(type) {
		case time.Time:
			values = append(values, v.Format("2006-01-02 15:04:05"))
		case float64:
			if isTimestampField(key) {
				t := time.Unix(int64(v), 0)
				if t.Year() < 1 || t.Year() > 9999 {
					log.Printf("WARNING: Invalid timestamp for field '%s': %v", key, v)
					t = time.Now()
				}
				values = append(values, t.Format("2006-01-02 15:04:05"))
			} else if isNumericField(key) {
				values = append(values, fmt.Sprintf("%.2f", v))
			} else {
				values = append(values, v)
			}
		case int64:
			if isTimestampField(key) {
				t := time.Unix(v, 0)
				if t.Year() < 1 || t.Year() > 9999 {
					log.Printf("WARNING: Invalid timestamp for field '%s': %v", key, v)
					t = time.Now()
				}
				values = append(values, t.Format("2006-01-02 15:04:05"))
			} else {
				values = append(values, v)
			}
		case models.Location:
			point := fmt.Sprintf("ST_SetSRID(ST_MakePoint(%f, %f), 4326)", v.Lon, v.Lat)
			values = append(values, point)
		case string:
			if key == "item_ids" {
				values = append(values, pq.Array(v))
			} else {
				values = append(values, v)
			}
		case []string:
			values = append(values, pq.Array(v))
		case []interface{}:
			if key == "item_ids" {
				strArr := make([]string, len(v))
				for i, item := range v {
					if str, ok := item.(string); ok {
						strArr[i] = str
					}
				}
				values = append(values, pq.Array(strArr))
			} else {
				values = append(values, pq.Array(v))
			}
		case map[string]interface{}:
			if isLocationMap(v) {
				lat, lon := getLocationCoords(v)
				values = append(values, fmt.Sprintf("SRID=4326;POINT(%f %f)", lon, lat))
			} else {
				jsonBytes, err := json.Marshal(v)
				if err != nil {
					log.Printf("Error marshaling JSON for key %s: %v", key, err)
					values = append(values, "{}")
				} else {
					values = append(values, string(jsonBytes))
				}
			}
		default:
			values = append(values, v)
		}

		columns = append(columns, snakeCaseKey(key))
		placeholderNum++
		placeholders = append(placeholders, fmt.Sprintf("$%d", placeholderNum))
	}

	return strings.Join(columns, ", "),
		values,
		strings.Join(placeholders, ", ")
}

func isLocationMap(m map[string]interface{}) bool {
	_, hasLat := m["lat"]
	_, hasLon := m["lon"]
	return hasLat && hasLon
}

func isNumericField(key string) bool {
	numericFields := map[string]bool{
		"prep_time":         true,
		"avg_prep_time":     true,
		"current_load":      true,
		"efficiency_score":  true,
		"pickup_efficiency": true,
		"speed":             true,
		"distance_covered":  true,
		"rating":            true,
		"total_ratings":     true,
	}
	return numericFields[key]
}

func isTimestampField(field string) bool {
	timestampFields := map[string]bool{
		"timestamp":               true,
		"update_time":             true,
		"estimated_arrival":       true,
		"actual_arrival":          true,
		"created_at":              true,
		"last_update_time":        true,
		"order_placed_at":         true,
		"prep_start_time":         true,
		"estimated_pickup_time":   true,
		"estimated_delivery_time": true,
		"pickup_time":             true,
		"next_check_time":         true,
		"in_transit_time":         true,
		"actual_delivery_time":    true,
		"updated_at":              true,
		"join_date":               true,
		"delivery_time":           true,
		"cancellation_time":       true,
		"ready_time":              true,
	}
	return timestampFields[field]
}

func generateRandomSuffix() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getLocationCoords(m map[string]interface{}) (float64, float64) {
	lat, _ := m["lat"].(float64)
	lon, _ := m["lon"].(float64)
	return lat, lon
}

func snakeCaseKey(key string) string {
	var result strings.Builder
	for i, r := range key {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}
