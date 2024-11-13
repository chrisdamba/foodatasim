package output

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/lib/pq"
	"log"
	"math"
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

	cols, vals, placeholders := buildInsertComponents(event)
	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		cols,
		placeholders,
	)

	_, err := p.db.Exec(query, vals...)
	if err != nil {
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
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("restaurants",
		"id", "host", "name", "currency", "phone",
		"town", "slug_name", "website_logo_url",
		"offline", "location", "cuisines", "rating",
		"total_ratings", "prep_time", "min_prep_time",
		"avg_prep_time", "pickup_efficiency", "capacity"))
	if err != nil {
		return err
	}

	for _, restaurant := range restaurants {
		point := fmt.Sprintf("POINT(%f %f)",
			restaurant.Location.Lon,
			restaurant.Location.Lat)

		_, err = stmt.Exec(
			restaurant.ID,
			restaurant.Host,
			restaurant.Name,
			restaurant.Currency,
			restaurant.Phone,
			restaurant.Town,
			restaurant.SlugName,
			restaurant.WebsiteLogoURL,
			restaurant.Offline,
			point,
			pq.Array(restaurant.Cuisines),
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

	if err = stmt.Close(); err != nil {
		return err
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
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for COPY operation
	stmt, err := tx.Prepare(pq.CopyIn(
		"menu_items",
		"id",
		"restaurant_id",
		"name",
		"description",
		"price",
		"prep_time",
		"category",
		"item_type",
		"popularity",
		"prep_complexity",
		"ingredients",
		"is_discount_eligible",
		"created_at",
		"updated_at",
	))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Current timestamp for created_at/updated_at
	now := time.Now()

	// Execute batch inserts
	for _, item := range menuItems {
		_, err = stmt.Exec(
			item.ID,
			item.RestaurantID,
			item.Name,
			nullableString(item.Description),
			item.Price,
			item.PrepTime,
			item.Category,
			item.Type,
			item.Popularity,
			item.PrepComplexity,
			pq.Array(item.Ingredients), // Use pq.Array for string array
			item.IsDiscountEligible,
			now, // created_at
			now, // updated_at
		)
		if err != nil {
			return fmt.Errorf("failed to exec statement for menu item %s: %w", item.ID, err)
		}
	}

	// Close the statement and flush the buffer
	err = stmt.Close()
	if err != nil {
		return fmt.Errorf("failed to close statement: %w", err)
	}

	// After inserting menu items, we might want to update the restaurant's menu count
	// This is optional but helps maintain denormalized counts
	err = p.updateRestaurantMenuCounts(tx)
	if err != nil {
		return fmt.Errorf("failed to update restaurant menu counts: %w", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresOutput) BatchInsertOrdersTx(tx *sql.Tx, orders []*models.Order) error {
	stmt, err := tx.Prepare(pq.CopyIn(
		"fact_order",
		"id", "customer_id", "restaurant_id", "delivery_partner_id",
		"total_amount", "delivery_cost", "order_placed_at",
		"prep_start_time", "estimated_pickup_time", "estimated_delivery_time",
		"pickup_time", "in_transit_time", "actual_delivery_time",
		"status", "payment_method", "delivery_address", "review_generated",
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, order := range orders {
		deliveryAddress := json.RawMessage(fmt.Sprintf(`{"lat":%f,"lon":%f}`,
			order.Address.Latitude, order.Address.Longitude))

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
			deliveryAddress,
			order.ReviewGenerated,
		)
		if err != nil {
			return err
		}
	}

	return stmt.Close()
}

func (p *PostgresOutput) UpdateRestaurantOrderCountsTx(tx *sql.Tx, orders []*models.Order) error {
	// Group orders by restaurant
	restaurantOrders := make(map[string]int)
	for _, order := range orders {
		restaurantOrders[order.RestaurantID]++
	}

	// Update counts for each restaurant
	for restaurantID, count := range restaurantOrders {
		_, err := tx.Exec(`
            UPDATE restaurants 
            SET total_orders = total_orders + $1,
                updated_at = NOW()
            WHERE id = $2
        `, count, restaurantID)
		if err != nil {
			return err
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

func nullableFloat64(f float64) sql.NullFloat64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{
		Float64: f,
		Valid:   true,
	}
}

func nullableInt64(i int64) sql.NullInt64 {
	if i == 0 { // or some other sentinel value
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}

func nullableBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{
		Bool:  *b,
		Valid: true,
	}
}

func timeFromNullable(nt sql.NullTime) time.Time {
	if !nt.Valid {
		return time.Time{} // zero time
	}
	return nt.Time
}

func stringFromNullable(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func float64FromNullable(nf sql.NullFloat64) float64 {
	if !nf.Valid {
		return 0.0
	}
	return nf.Float64
}

func int64FromNullable(ni sql.NullInt64) int64 {
	if !ni.Valid {
		return 0
	}
	return ni.Int64
}

func boolFromNullable(nb sql.NullBool) bool {
	if !nb.Valid {
		return false
	}
	return nb.Bool
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

func locationToPoint(loc models.Location) string {
	return fmt.Sprintf("POINT(%f %f)", loc.Lon, loc.Lat)
}

func topicToTable(topic string) string {
	tableMap := map[string]string{
		// fact Tables
		"order_placed_events":       "fact_order",
		"order_preparation_events":  "fact_order",
		"order_ready_events":        "fact_order",
		"order_pickup_events":       "fact_order",
		"order_delivery_events":     "fact_order",
		"order_cancellation_events": "fact_order",
		"order_in_transit_events":   "fact_order",

		"delivery_status_check_events": "fact_delivery_performance",
		"partner_location_events":      "fact_delivery_performance",
		"delivery_partner_events":      "fact_delivery_performance",

		"restaurant_status_events":  "fact_restaurant_performance",
		"restaurant_metrics_events": "fact_restaurant_performance",

		"review_events": "fact_review",

		// Dimension Tables
		"user_behaviour_events":  "dim_customer",
		"user_preference_events": "dim_customer",

		"restaurant_menu_events":     "dim_restaurant",
		"restaurant_capacity_events": "dim_restaurant",
		"restaurant_hours_events":    "dim_restaurant",

		"delivery_partner_status_events": "dim_delivery_partner",
		"delivery_partner_shift_events":  "dim_delivery_partner",

		// Time and Location based events
		"traffic_condition_events": "fact_traffic_condition",
		"weather_condition_events": "fact_weather_condition",
		"peak_hour_events":         "fact_peak_hours",

		// Menu related events
		"menu_item_events":         "dim_menu_item",
		"menu_price_events":        "fact_menu_price",
		"menu_availability_events": "fact_menu_availability",

		// Promotion and discount events
		"promotion_events": "fact_promotion",
		"discount_events":  "fact_discount",

		// Payment events
		"payment_events": "fact_payment",
		"refund_events":  "fact_refund",

		// Service metrics events
		"service_quality_events":       "fact_service_quality",
		"delivery_time_events":         "fact_delivery_time",
		"customer_satisfaction_events": "fact_customer_satisfaction",

		// Cost and revenue events
		"revenue_events":    "fact_revenue",
		"cost_events":       "fact_cost",
		"commission_events": "fact_commission",

		// Operational events
		"capacity_utilization_events": "fact_capacity_utilization",
		"efficiency_metrics_events":   "fact_efficiency_metrics",
		"performance_metrics_events":  "fact_performance_metrics",

		// Integration events
		"notification_events":  "fact_notification",
		"communication_events": "fact_communication",
		"system_events":        "fact_system_log",
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
	// store columns and values in sorted order for consistent queries
	var columns []string
	var values []interface{}
	var placeholderNum int
	var placeholders []string

	// get sorted keys to ensure consistent order
	keys := make([]string, 0, len(event))
	for k := range event {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// build columns and values arrays
	for _, key := range keys {
		val := event[key]

		// special handling for different types
		switch v := val.(type) {
		case time.Time:
			values = append(values, v)
		case models.Location:
			// convert location to PostGIS point
			values = append(values, fmt.Sprintf("POINT(%f %f)", v.Lon, v.Lat))
		case []string:
			values = append(values, pq.Array(v))
		case map[string]interface{}:
			// convert maps to JSONB
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				log.Printf("Error marshaling JSON for key %s: %v", key, err)
				continue
			}
			values = append(values, string(jsonBytes))
		default:
			values = append(values, v)
		}

		// add column name and placeholder
		columns = append(columns, snakeCaseKey(key))
		placeholderNum++
		placeholders = append(placeholders, fmt.Sprintf("$%d", placeholderNum))
	}

	return strings.Join(columns, ", "),
		values,
		strings.Join(placeholders, ", ")
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
