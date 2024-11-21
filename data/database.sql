-- Connect to postgres database to create new database if it doesn't exist

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'foodatasim') THEN
        CREATE DATABASE foodatasim;
END IF;
END
$$;

-- drop database foodatasim with(force);

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
                       order_frequency DOUBLE PRECISION NOT NULL
);


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

-- -- Create order items junction table
-- DROP TABLE IF EXISTS order_items;
-- CREATE TABLE order_items (
--     order_id VARCHAR(255) NOT NULL,
--     menu_item_id VARCHAR(255) NOT NULL,
--     quantity INTEGER NOT NULL CHECK (quantity > 0),
--     unit_price DECIMAL(10,2) NOT NULL CHECK (unit_price >= 0),
--     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--     PRIMARY KEY (order_id, menu_item_id),
--     FOREIGN KEY (order_id) REFERENCES orders(id),
--     FOREIGN KEY (menu_item_id) REFERENCES menu_items(id)
-- );

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

-- Create trigger functions for timestamp updates
-- CREATE TRIGGER update_orders_updated_at
--     BEFORE UPDATE ON orders
--     FOR EACH ROW
--     EXECUTE FUNCTION update_updated_at_column();

-- CREATE TRIGGER update_reviews_updated_at
--     BEFORE UPDATE ON reviews
--     FOR EACH ROW
--     EXECUTE FUNCTION update_updated_at_column();

-- -- Create views for analytics
-- CREATE MATERIALIZED VIEW order_statistics AS
-- SELECT
--     restaurant_id,
--     DATE_TRUNC('day', order_placed_at) as order_date,
--     COUNT(*) as total_orders,
--     SUM(total_amount) as total_revenue,
--     AVG(EXTRACT(EPOCH FROM (actual_delivery_time - order_placed_at)))/60 as avg_delivery_time_minutes
-- FROM orders
-- WHERE status = 'delivered'
-- GROUP BY restaurant_id, DATE_TRUNC('day', order_placed_at);

-- CREATE MATERIALIZED VIEW restaurant_ratings AS
-- SELECT
--     restaurant_id,
--     COUNT(*) as total_reviews,
--     AVG(food_rating) as avg_food_rating,
--     AVG(overall_rating) as avg_overall_rating
-- FROM reviews
-- WHERE NOT is_ignored
-- GROUP BY restaurant_id;

-- -- Create function to refresh materialized views
-- CREATE OR REPLACE FUNCTION refresh_order_analytics()
-- RETURNS TRIGGER AS $$
-- BEGIN
--     REFRESH MATERIALIZED VIEW CONCURRENTLY order_statistics;
--     REFRESH MATERIALIZED VIEW CONCURRENTLY restaurant_ratings;
--     RETURN NULL;
-- END;
-- $$ LANGUAGE plpgsql;

-- -- Create triggers to refresh materialized views
-- CREATE TRIGGER refresh_order_analytics_trigger
-- AFTER INSERT OR UPDATE OR DELETE ON orders
-- FOR EACH STATEMENT
-- EXECUTE FUNCTION refresh_order_analytics();

-- CREATE TRIGGER refresh_restaurant_ratings_trigger
-- AFTER INSERT OR UPDATE OR DELETE ON reviews
-- FOR EACH STATEMENT
-- EXECUTE FUNCTION refresh_order_analytics();

