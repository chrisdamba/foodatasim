-- Connect to postgres database to create new database if it doesn't exist

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'foodatasim') THEN
        CREATE DATABASE foodatasim;
END IF;
END
$$;


-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
DROP TABLE IF EXISTS users;
CREATE TABLE users (
                       id VARCHAR(255) PRIMARY KEY,
                       name VARCHAR(255) NOT NULL,
                       email VARCHAR(255) NOT NULL,
                       join_date TIMESTAMP NOT NULL,
                       location geography(Point, 4326) NOT NULL,
                       preferences TEXT[], -- Using array type for string arrays
                       dietary_restrictions TEXT[],
                       order_frequency DOUBLE PRECISION NOT NULL,
                       segment VARCHAR(50) DEFAULT 'regular',
                       behavior_profile JSONB,
                       purchase_patterns JSONB,
                       order_history JSONB,
                       lifetime_orders INTEGER DEFAULT 0,
                       lifetime_spend DECIMAL(10,2) DEFAULT 0.0
);

CREATE INDEX idx_users_segment ON users(segment);
CREATE INDEX idx_users_behavior ON users USING GIN(behavior_profile);
CREATE INDEX idx_users_order_history ON users USING GIN(order_history);


-- Create enum for offline status
CREATE TYPE offline_status AS ENUM (
    'ENABLED',
    'DISABLED'
);

-- Create restaurants table
DROP TABLE IF EXISTS restaurants;
CREATE TABLE restaurants (
                             id VARCHAR(255) PRIMARY KEY,
                             host VARCHAR(255) NOT NULL,
                             name VARCHAR(255) NOT NULL,
                             currency INTEGER NOT NULL,
                             phone VARCHAR(50),
                             town VARCHAR(255),
                             slug_name VARCHAR(255) NOT NULL,
                             website_logo_url TEXT,
                             offline offline_status NOT NULL DEFAULT 'DISABLED',
                             location geography(Point, 4326) NOT NULL,
                             cuisines TEXT[] NOT NULL,
                             rating DOUBLE PRECISION NOT NULL CHECK (rating >= 1 AND rating <= 5),
                             total_ratings DOUBLE PRECISION NOT NULL DEFAULT 0,
                             prep_time DOUBLE PRECISION NOT NULL,
                             min_prep_time DOUBLE PRECISION NOT NULL,
                             avg_prep_time DOUBLE PRECISION NOT NULL,
                             pickup_efficiency DOUBLE PRECISION NOT NULL,
                             capacity INTEGER NOT NULL CHECK (capacity > 0),
                             price_tier VARCHAR(50) DEFAULT 'standard',
                             reputation_metrics JSONB,
                             reputation_history JSONB[],
                             demand_patterns JSONB,
                             market_position JSONB,
                             popularity_metrics JSONB,
                             created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                             updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


-- Create indexes for common query patterns
CREATE INDEX idx_restaurants_location ON restaurants USING GIST(location);
CREATE INDEX idx_restaurants_cuisines ON restaurants USING GIN(cuisines);
CREATE INDEX idx_restaurants_rating ON restaurants(rating);
CREATE INDEX idx_restaurants_town ON restaurants(town);
CREATE INDEX idx_restaurants_slug_name ON restaurants(slug_name);
CREATE INDEX idx_restaurants_offline ON restaurants(offline);
CREATE INDEX idx_restaurants_reputation ON restaurants USING GIN(reputation_metrics);
CREATE INDEX idx_restaurants_price_tier ON restaurants(price_tier);

-- Create unique indexes to enforce business rules
CREATE UNIQUE INDEX idx_restaurants_slug_name_unique ON restaurants(slug_name);
CREATE UNIQUE INDEX idx_restaurants_phone_unique ON restaurants(phone);



-- Enum for partner status
CREATE TYPE partner_status AS ENUM (
    'available',
    'en_route_to_pickup',
    'en_route_to_delivery'
);

-- Delivery Partners table
DROP TABLE IF EXISTS delivery_partners;
CREATE TABLE delivery_partners (
                                   id VARCHAR(255) PRIMARY KEY,
                                   name VARCHAR(255) NOT NULL,
                                   join_date TIMESTAMP NOT NULL,
                                   rating DOUBLE PRECISION NOT NULL CHECK (rating >= 1 AND rating <= 5),
                                   total_ratings DOUBLE PRECISION NOT NULL DEFAULT 0,
                                   experience DOUBLE PRECISION NOT NULL CHECK (experience >= 0 AND experience <= 1),
                                   speed DOUBLE PRECISION NOT NULL,
                                   avg_speed DOUBLE PRECISION NOT NULL,
                                   current_order_id VARCHAR(255),
                                   current_location geography(Point, 4326) NOT NULL,
                                   status partner_status NOT NULL,
                                   last_update_time TIMESTAMP NOT NULL,
                                   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                   updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for common query patterns
CREATE INDEX idx_delivery_partners_status ON delivery_partners(status);
CREATE INDEX idx_delivery_partners_current_order ON delivery_partners(current_order_id);
CREATE INDEX idx_delivery_partners_location ON delivery_partners USING GIST(current_location);


-- Create enum for menu item types
CREATE TYPE menu_item_type AS ENUM (
    'appetizer',
    'main course',
    'side dish',
    'dessert',
    'drink'
);

-- Create menu items table
DROP TABLE IF EXISTS menu_items;
CREATE TABLE menu_items (
                            id VARCHAR(255) PRIMARY KEY,
                            restaurant_id VARCHAR(255) NOT NULL,
                            name VARCHAR(255) NOT NULL,
                            description TEXT,
                            price DECIMAL(10,2) NOT NULL CHECK (price > 0),
                            prep_time DOUBLE PRECISION NOT NULL CHECK (prep_time >= 0),
                            category VARCHAR(255) NOT NULL,
                            type menu_item_type NOT NULL,
                            popularity DOUBLE PRECISION NOT NULL CHECK (popularity >= 0 AND popularity <= 1),
                            prep_complexity DOUBLE PRECISION NOT NULL CHECK (prep_complexity >= 0 AND prep_complexity <= 1),
                            ingredients TEXT[] NOT NULL,
                            is_discount_eligible BOOLEAN NOT NULL DEFAULT false,
                            base_price DECIMAL(10,2),
                            dynamic_pricing BOOLEAN DEFAULT false,
                            price_history JSONB[],
                            seasonal_factor DECIMAL(5,2) DEFAULT 1.0,
                            time_based_demand JSONB,
                            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                            FOREIGN KEY (restaurant_id) REFERENCES restaurants(id)
);

-- Create indexes for common query patterns
CREATE INDEX idx_menu_items_restaurant_id ON menu_items(restaurant_id);
CREATE INDEX idx_menu_items_type ON menu_items(type);
CREATE INDEX idx_menu_items_category ON menu_items(category);
CREATE INDEX idx_menu_items_price ON menu_items(price);
CREATE INDEX idx_menu_items_popularity ON menu_items(popularity);
CREATE INDEX idx_menu_items_ingredients ON menu_items USING GIN(ingredients);
CREATE INDEX idx_menu_items_dynamic_pricing ON menu_items(dynamic_pricing);
CREATE INDEX idx_menu_items_price_history ON menu_items USING GIN(price_history);

-- Create enums for order status and payment methods
CREATE TYPE order_status AS ENUM (
    'placed',
    'preparing',
    'in_transit',
    'delivered',
    'cancelled'
);

CREATE TYPE payment_method AS ENUM (
    'card',
    'cash',
    'wallet'
);


-- Create orders table
DROP TABLE IF EXISTS orders;
CREATE TABLE orders (
                        id VARCHAR(255) PRIMARY KEY,
                        customer_id VARCHAR(255) NOT NULL,
                        restaurant_id VARCHAR(255) NOT NULL,
                        delivery_partner_id VARCHAR(255),
                        total_amount DECIMAL(10,2) NOT NULL CHECK (total_amount >= 0),
                        delivery_cost DECIMAL(10,2) NOT NULL CHECK (delivery_cost >= 0),
                        order_placed_at TIMESTAMP NOT NULL,
                        status order_status NOT NULL DEFAULT 'placed',
                        payment_method payment_method NOT NULL,
                        delivery_address JSONB NOT NULL,
                        review_generated BOOLEAN NOT NULL DEFAULT false,

                        item_ids TEXT[],
                        prep_start_time TIMESTAMP,
                        estimated_pickup_time TIMESTAMP,
                        estimated_delivery_time TIMESTAMP,
                        pickup_time TIMESTAMP,
                        in_transit_time TIMESTAMP,
                        actual_delivery_time TIMESTAMP,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        FOREIGN KEY (customer_id) REFERENCES users(id),
                        FOREIGN KEY (restaurant_id) REFERENCES restaurants(id),
                        FOREIGN KEY (delivery_partner_id) REFERENCES delivery_partners(id)
);


-- Create reviews table
DROP TABLE IF EXISTS reviews;
CREATE TABLE reviews (
                         id VARCHAR(255) PRIMARY KEY,
                         order_id VARCHAR(255) NOT NULL UNIQUE,
                         customer_id VARCHAR(255) NOT NULL,
                         restaurant_id VARCHAR(255) NOT NULL,
                         delivery_partner_id VARCHAR(255) NOT NULL,
                         food_rating DECIMAL(2,1) NOT NULL CHECK (food_rating >= 1 AND food_rating <= 5),
                         delivery_rating DECIMAL(2,1) NOT NULL CHECK (delivery_rating >= 1 AND delivery_rating <= 5),
                         overall_rating DECIMAL(2,1) NOT NULL CHECK (overall_rating >= 1 AND overall_rating <= 5),
                         comment TEXT,
                         is_ignored BOOLEAN NOT NULL DEFAULT false,
                         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                         updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                         FOREIGN KEY (order_id) REFERENCES orders(id),
                         FOREIGN KEY (customer_id) REFERENCES users(id),
                         FOREIGN KEY (restaurant_id) REFERENCES restaurants(id),
                         FOREIGN KEY (delivery_partner_id) REFERENCES delivery_partners(id)
);

-- Create indexes for orders
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_restaurant_id ON orders(restaurant_id);
CREATE INDEX idx_orders_delivery_partner_id ON orders(delivery_partner_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_placed_at ON orders(order_placed_at);
CREATE INDEX idx_orders_delivery_time ON orders(actual_delivery_time);

-- Create indexes for reviews
CREATE INDEX idx_reviews_customer_id ON reviews(customer_id);
CREATE INDEX idx_reviews_restaurant_id ON reviews(restaurant_id);
CREATE INDEX idx_reviews_delivery_partner_id ON reviews(delivery_partner_id);
CREATE INDEX idx_reviews_overall_rating ON reviews(overall_rating);
CREATE INDEX idx_reviews_created_at ON reviews(created_at);


-- Create a view for restaurant performance metrics
CREATE OR REPLACE VIEW restaurant_performance_metrics AS
SELECT
    r.id,
    r.name,
    r.price_tier,
    r.rating,
    r.reputation_metrics->>'consistency_score' as consistency_score,
    r.reputation_metrics->>'reliability_score' as reliability_score,
    r.reputation_metrics->>'price_quality_score' as price_quality_score,
    r.market_position->>'popularity' as popularity,
    COUNT(o.id) as order_count,
    AVG(o.total_amount) as avg_order_value,
    COUNT(DISTINCT o.customer_id) as unique_customers,
    AVG(rev.overall_rating) as avg_recent_rating
FROM restaurants r
    LEFT JOIN orders o ON r.id = o.restaurant_id
    AND o.order_placed_at >= NOW() - INTERVAL '30 days'
    LEFT JOIN reviews rev ON o.id = rev.order_id
GROUP BY r.id, r.name, r.price_tier, r.rating,
    r.reputation_metrics, r.market_position;

-- Create a view for user behavior analysis
CREATE OR REPLACE VIEW user_behavior_analysis AS
SELECT
    u.id,
    u.segment,
    u.lifetime_orders,
    u.lifetime_spend,
    u.behavior_profile->>'order_timing_preference' as preferred_times,
    u.behavior_profile->'price_thresholds'->>'target' as target_price,
    COUNT(o.id) as orders_last_30_days,
    AVG(o.total_amount) as avg_order_value,
    ARRAY_AGG(DISTINCT r.price_tier) as preferred_restaurant_tiers,
    ARRAY_AGG(DISTINCT mi.category) as preferred_categories
FROM users u
    LEFT JOIN orders o ON u.id = o.customer_id
    AND o.order_placed_at >= NOW() - INTERVAL '30 days'
    LEFT JOIN restaurants r ON o.restaurant_id = r.id
    LEFT JOIN LATERAL (
    SELECT DISTINCT unnest(o.item_ids) as item_id
    ) items ON true
    LEFT JOIN menu_items mi ON items.item_id = mi.id
GROUP BY u.id, u.segment, u.lifetime_orders, u.lifetime_spend,
    u.behavior_profile;

-- Create a function to update reputation metrics
CREATE OR REPLACE FUNCTION update_restaurant_reputation()
RETURNS TRIGGER AS $$
BEGIN
    -- Update reputation history
    NEW.reputation_history = array_append(
        NEW.reputation_history,
        NEW.reputation_metrics
    );

    -- Keep only last 90 days of history
    NEW.reputation_history = array_remove(
        NEW.reputation_history,
        array_to_json(
            array_agg(h)
        )::jsonb
    ) FROM unnest(NEW.reputation_history) h
    WHERE (h->>'last_update')::timestamp < NOW() - INTERVAL '90 days';

RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_restaurant_reputation_trigger
    BEFORE UPDATE ON restaurants
    FOR EACH ROW
    WHEN (NEW.reputation_metrics IS DISTINCT FROM OLD.reputation_metrics)
EXECUTE FUNCTION update_restaurant_reputation();
