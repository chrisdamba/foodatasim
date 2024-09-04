foodatasim
=======

FoodataSim (Food Data Simulator) is a program that generates synthetic event data for food delivery platforms. Written in Go, it's designed to simulate user behaviour, restaurant operations, and delivery processes for a fictional food delivery service. The generated data looks like real usage patterns but is entirely synthetic. You can configure the program to create data for various scenarios: from a small number of users over a few hours to a large user base over several years. The data can be written to files or streamed to Apache Kafka.

Use the synthetic data for product development, testing, demos, performance evaluation, training, or any situation where a stream of realistic food delivery data is beneficial.

Disclaimer: Note that this data should not be used for actual machine learning algorithm research or to draw conclusions about real-world user behaviour/analysis.


## Project Structure

```
foodata/
│
├── cmd/foodata/          # Main application entry point
├── internal/             # Internal application code
│   ├── config/           # Configuration handling
│   ├── model/            # Data models
│   ├── simulator/        # Core simulation logic
│   └── output/           # Output formatting (JSON, CSV, Kafka)
├── pkg/                  # Public libraries
├── examples/             # Example configuration files
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

Statistical Model
=================

Foodatasim's simulator is based on observations of real food delivery platform usage patterns. It aims to create data that mirrors real-world behaviour: users order at varying frequencies, restaurant popularity fluctuates, order volumes change based on time of day and day of week, and delivery times are influenced by various factors.

Key features of the statistical model include:

* User arrivals follow a Poisson process, modified by time-of-day and day-of-week factors
* Order placement times follow a log-normal distribution
* Restaurant selection is influenced by popularity, user preferences, and time of day
* Delivery times are based on restaurant preparation times, distance, and simulated traffic conditions
* User ratings follow a distribution that reflects common rating patterns on delivery platforms

How the simulation works
========================

When you run Foodatasim, it starts by generating a set of users, restaurants, and delivery partners with randomly assigned properties. This includes attributes like names and locations, as well as behavioral characteristics like ordering frequency and cuisine preferences.

You need to specify a configuration file (samples are included in `examples`). This file defines parameters for generating sessions, restaurant operations, and delivery processes. The simulator also loads data files that describe distributions for various parameters (such as cuisines, dish names, and user agents).

The simulator operates by maintaining a priority queue of events, ordered by timestamp. It processes each event, generates the necessary data, and then determines the next event for each entity (user, restaurant, or delivery partner), adding it back to the queue.

The simulation takes into account various factors when generating events:

* Time of day and day of week influence order volumes and restaurant selection
* User preferences affect restaurant and dish selection
* Restaurant popularity and ratings influence order frequency
* Delivery times are affected by simulated traffic conditions and distance


## Configuration

The config file is a JSON file with key-value pairs. Here's an explanation of key parameters:

* `seed`: Seed for the pseudo-random number generator
* `start_date`: Start date for data generation (ISO8601 format)
* `end_date`: End date for data generation (ISO8601 format)
* `initial_users`: Initial number of users
* `initial_restaurants`: Number of restaurants
* `initial_partners`: Number of delivery partners
* `user_growth_rate`: Annual growth rate for users
* `partner_growth_rate`: Annual growth rate for delivery partners
* `order_frequency`: Average number of orders per user per day
* `peak_hour_factor`: Factor to increase order frequency during peak hours
* `weekend_factor`: Factor to adjust order frequency on weekends
* `traffic_variability`: Factor to add randomness to traffic conditions

Example config file:

```json
{
  "seed": 42,
  "start_date": "2024-06-01T00:00:00Z",
  "end_date": "2024-07-01T00:00:00Z",
  "initial_users": 1000,
  "initial_restaurants": 100,
  "initial_partners": 50,
  "user_growth_rate": 0.01,
  "partner_growth_rate": 0.02,
  "order_frequency": 0.1,
  "peak_hour_factor": 1.5,
  "weekend_factor": 1.2,
  "traffic_variability": 0.2,
  "kafka_enabled": false,
  "kafka_broker_list": "localhost:9092",
  "output_file_path": "",
  "continuous": false
}
```

Usage
=====

To build the executable, run:

    $ go build -o bin/foodatasim

The program accepts several command-line options:

    $ bin/foodatasim --help
        --config string      config file (default is $HOME/.foodatasim.yaml)
        --seed int           Random seed for simulation (default 42)
        --start-date string  Start date for simulation (default "current date")
        --end-date string    End date for simulation (default "current date + 1 month")
        --initial-users int  Initial number of users (default 1000)
        --initial-restaurants int  Initial number of restaurants (default 100)
        --initial-partners int     Initial number of delivery partners (default 50)
        --user-growth-rate float   Annual user growth rate (default 0.01)
        --partner-growth-rate float  Annual partner growth rate (default 0.02)
        --order-frequency float      Average order frequency per user per day (default 0.1)
        --peak-hour-factor float     Factor for increased order frequency during peak hours (default 1.5)
        --weekend-factor float       Factor for increased order frequency during weekends (default 1.2)
        --traffic-variability float  Variability in traffic conditions (default 0.2)
        --kafka-enabled              Enable Kafka output
        --kafka-broker-list string   Kafka broker list (default "localhost:9092")
        --output-file string         Output file path (if not using Kafka)
        --continuous                 Run simulation in continuous mode

Example for generating about 1 million events (1,000 users for a month, growing at 1% annually):

    $ bin/foodatasim --config config.json --start-date "2023-06-01T00:00:00Z" --end-date "2023-07-01T00:00:00Z" --initial-users 1000 --user-growth-rate 0.01 --output-file data/synthetic.json

## Generated Data

The simulator generates data for:

1. Users: ID, name, join date, location, preferences, order frequency
2. Restaurants: ID, name, location, cuisines, rating, preparation time, pickup efficiency
3. Menu Items: ID, restaurant ID, name, description, price, preparation time, category
4. Delivery Partners: ID, name, join date, rating, current location, status, experience
5. Orders: ID, user ID, restaurant ID, delivery partner ID, items (list of menu item IDs), total amount, timestamps, status
6. Traffic Conditions: Time, location, density

## Using the Data for GNN Models

The generated data can be used to train Graph Neural Network (GNN) models for various tasks in the food delivery ecosystem:

1. Optimal restaurant rankings for delivery partners
2. Predicting delivery partner utilization in different areas and times
3. Estimating preparation and delivery times
4. Improving overall efficiency of the three-sided marketplace

To use the data for GNN models:

1. Generate a dataset using foodatasim
2. Convert the output into a graph representation, where nodes are users, restaurants, and delivery partners, and edges represent interactions (orders, deliveries)
3. Use the graph data to train and evaluate GNN models for your specific task


Building large datasets in parallel
===================================
For generating large datasets quickly, you can run multiple instances of foodatasim simultaneously:

* Use different random seeds for each instance
* Assign non-overlapping user ID ranges to each instance
* Create separate configuration files for each instance
* Generate data for the same time period across all instances

A Cool Example
==============

To simulate A/B tests, create multiple datasets for the same time period with different sets of user IDs, different tags, and varied parameters. For example:

    $ bin/foodata -c "examples/control-config.json" --tag control -n 5000 \
      -s "2023-06-01" -e "2023-09-01" -g 0.25 --seed 1 control-data.json

    $ bin/foodata -c "examples/test-config.json" --tag test -n 5000 \
      -s "2023-06-01" -e "2023-09-01" -g 0.25 --seed 2 test-data.json

This generates two datasets with different characteristics, allowing you to simulate and analyze the effects of different configurations or features.

Issues and Future Work
======================

Contributions are welcome! Here are some ideas for improvements:

* Implement multi-threading for faster data generation
* Refine the statistical models to more accurately reflect real-world patterns
* Add more configurable parameters to simulate diverse scenarios
* Implement more sophisticated traffic and weather models
* Add support for simulating marketing campaigns and their effects

License
=======
This project is licensed under the MIT License (see the LICENSE.txt file for details).

About the source data
=====================

While the generated data is synthetic, it's based on observations of real food delivery platforms. The restaurant names, cuisines, and dish names are randomly generated but designed to reflect common patterns in the food industry.

For the real novice
===================

If you're new to Go, here are some steps to get started:

1. Install Go from https://golang.org/
2. Clone this repository
3. Navigate to the project directory
4. Run `go build -o bin/foodata` to build the executable
5. Run `./bin/foodata --help` to see available options

For more information on using Go, check out the official Go documentation at https://golang.org/doc/
