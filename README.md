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
  "initial_users": 10000,
  "initial_restaurants": 1000,
  "initial_partners": 300,
  "user_growth_rate": 0.01,
  "partner_growth_rate": 0.02,
  "order_frequency": 0.1,
  "peak_hour_factor": 1.5,
  "weekend_factor": 1.2,
  "traffic_variability": 0.2,
  "kafka_enabled": true,
  "kafka_use_local": true,
  "kafka_broker_list": "localhost:9092",
  "output_destination": "s3",
  "output_format": "parquet",
  "output_path": "output",
  "output_folder": "events",
  "continuous": true,
  "cloud_storage": {
    "provider": "s3",
    "bucket_name": "foodsimdata",
    "region": "eu-west-2"
  },
  "city_name": "Stoke-on-Trent",
  "default_currency": 1,
  "min_prep_time": 10,
  "max_prep_time": 60,
  "min_rating": 1.0,
  "max_rating": 5.0,
  "max_initial_ratings": 100,
  "min_efficiency": 0.5,
  "max_efficiency": 1.5,
  "min_capacity": 5,
  "max_capacity": 50,
  "tax_rate": 0.08,
  "service_fee_percentage": 0.15,
  "discount_percentage": 0.1,
  "min_order_for_discount": 20.0,
  "max_discount_amount": 10.0,
  "base_delivery_fee": 2.99,
  "free_delivery_threshold": 30.0,
  "small_order_threshold": 10.0,
  "small_order_fee": 2.0,
  "restaurant_rating_alpha": 0.1,
  "partner_rating_alpha": 0.1,
  "near_location_threshold": 50.0,
  "city_latitude": 53.002666,
  "city_longitude": -2.179404,
  "urban_radius": 10.0,
  "hotspot_radius": 2.0,
  "partner_move_speed": 40.0,
  "location_precision": 0.0001,
  "user_behaviour_window": 10,
  "restaurant_load_factor": 0.2,
  "efficiency_adjust_rate": 0.1
}
```

## Usage

### Building the Executable

To build the executable, run:

```bash
go build -o bin/foodatasim
```

### Running the Simulator

The program accepts several command-line options:

```bash
./bin/foodatasim --help
```

Available options:

- `--config string`: Config file (default is `./examples/config.json`).
- `--seed int`: Random seed for simulation (default 42).
- `--start-date string`: Start date for simulation (default "current date").
- `--end-date string`: End date for simulation (default "current date + 1 month").
- `--initial-users int`: Initial number of users (default 1000).
- `--initial-restaurants int`: Initial number of restaurants (default 100).
- `--initial-partners int`: Initial number of delivery partners (default 50).
- `--user-growth-rate float`: Annual user growth rate (default 0.01).
- `--partner-growth-rate float`: Annual partner growth rate (default 0.02).
- `--order-frequency float`: Average order frequency per user per day (default 0.1).
- `--peak-hour-factor float`: Factor for increased order frequency during peak hours (default 1.5).
- `--weekend-factor float`: Factor for increased order frequency during weekends (default 1.2).
- `--traffic-variability float`: Variability in traffic conditions (default 0.2).
- `--kafka-enabled`: Enable Kafka output.
- `--kafka-broker-list string`: Kafka broker list (default "localhost:9092").
- `--output-file string`: Output file path (if not using Kafka).
- `--continuous`: Run simulation in continuous mode.

Example for generating about 1 million events (1,000 users for a month, growing at 1% annually):

```bash
./bin/foodatasim --config examples/config.json --start-date "2024-06-01T00:00:00Z" --end-date "2024-07-01T00:00:00Z" --initial-users 1000 --user-growth-rate 0.01 --output-file data/synthetic.json
```


## Building the Docker Image

To build a Docker image for FoodataSim, you can use Docker's Buildx tool, which allows you to build images for multiple platforms (e.g., `amd64` and `arm64`). This is especially useful if you plan to run the image on different architectures.

### Prerequisites

- **Docker Buildx**: Ensure you have Docker Buildx installed and enabled. It comes bundled with Docker Desktop and can be enabled in the Docker CLI.
- **Docker Hub Account**: You'll need an account on Docker Hub (or another container registry) to push the image.

### Building and Pushing the Image

Use the following command to build and push the Docker image:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t your-dockerhub-username/foodatasim:latest \
  --push .
```

**Explanation:**

- `docker buildx build`: Uses Docker Buildx for extended build capabilities.
- `--platform linux/amd64,linux/arm64`: Specifies the target platforms to build for.
- `-t your-dockerhub-username/foodatasim:latest`: Tags the image with your Docker Hub username and the image name `foodatasim:latest`.
- `--push`: Pushes the built image to the specified registry (e.g., Docker Hub).
- `.`: Specifies the build context (the current directory).

### Steps to Build and Push

1. **Enable Buildx (if not already enabled):**

   If you're using Docker Desktop, Buildx is included by default. To enable Buildx, you can run:

   ```bash
   docker buildx create --name mybuilder --use
   docker buildx inspect --bootstrap
   ```

2. **Log in to Docker Hub:**

   ```bash
   docker login
   ```

   Enter your Docker Hub username and password when prompted.

3. **Build and Push the Image:**

   Run the build command, replacing `your-dockerhub-username` with your actual Docker Hub username:

   ```bash
   docker buildx build --platform linux/amd64,linux/arm64 \
     -t your-dockerhub-username/foodatasim:latest \
     --push .
   ```

4. **Verify the Image on Docker Hub:**

   After the build and push complete, you can verify the image is available on Docker Hub.

### Building for a Single Platform (Optional)

If you only need to build for your local machine's architecture, you can simplify the command:

```bash
docker build -t your-dockerhub-username/foodatasim:latest .
```

## Running the Docker Container

After building and pushing the Docker image, you can run FoodataSim using Docker. Below are detailed instructions for different scenarios, including enabling Kafka with local setup and using Confluent Cloud.

### Running with Kafka Enabled and `use_local=true` (Local Kafka)

This configuration streams simulated data to a locally running Kafka broker.

1. **Ensure Kafka is Running Locally:**

   Make sure you have a Kafka broker running on your local machine (e.g., on `localhost:9092`).

2. **Run the Docker Container:**

   ```bash
   docker run -d \
     --name foodatasim \
     -e FOODATASIM_KAFKA_ENABLED=true \
     -e FOODATASIM_KAFKA_USE_LOCAL=true \
     -e FOODATASIM_KAFKA_BROKER_LIST="localhost:9092" \
     your-dockerhub-username/foodatasim:latest
   ```

   **Explanation of Environment Variables:**

   - `FOODATASIM_KAFKA_ENABLED=true`: Enables Kafka output.
   - `FOODATASIM_KAFKA_USE_LOCAL=true`: Configures the simulator to use the local Kafka broker.
   - `FOODATASIM_KAFKA_BROKER_LIST="localhost:9092"`: Specifies the Kafka broker address.

3. **Verify the Container is Running:**

   ```bash
   docker ps
   ```

   You should see an entry similar to:

   ```
   CONTAINER ID   IMAGE                           COMMAND         CREATED          STATUS          PORTS                    NAMES
   abcdef123456   your-dockerhub-username/foodatasim:latest   "./foodatasim"   10 seconds ago   Up 8 seconds    0.0.0.0:9092->9092/tcp   foodatasim
   ```

4. **Check Logs for Confirmation:**

   ```bash
   docker logs -f foodatasim
   ```

   Look for logs indicating that Kafka output is enabled and messages are being produced.

### Running with Kafka Enabled and `use_local=false` (Confluent Cloud)

This configuration streams simulated data to Confluent Cloud Kafka. You'll need to provide Confluent Cloud credentials via environment variables.

1. **Set Up Confluent Cloud Kafka:**

   - **Create a Confluent Cloud Account:** If you don't have one, sign up at [Confluent Cloud](https://www.confluent.io/confluent-cloud/).
   - **Create a Kafka Cluster:** Follow Confluent Cloud's documentation to set up a Kafka cluster.
   - **Obtain Kafka Credentials:**
      - **API Key and Secret:** For SASL authentication.
      - **Bootstrap Servers:** The address of your Kafka brokers.
      - **Security Protocol and SASL Mechanism:** Typically `SASL_SSL` and `PLAIN`.

2. **Prepare Environment Variables:**

   You'll need to pass the following environment variables to the Docker container:

   - `FOODATASIM_KAFKA_ENABLED=true`
   - `FOODATASIM_KAFKA_USE_LOCAL=false`
   - `FOODATASIM_KAFKA_BROKER_LIST=<Bootstrap Servers>` (e.g., `pkc-4wxyz.us-west1.gcp.confluent.cloud:9092`)
   - `FOODATASIM_KAFKA_SECURITY_PROTOCOL=SASL_SSL`
   - `FOODATASIM_KAFKA_SASL_MECHANISM=PLAIN`
   - `FOODATASIM_KAFKA_SASL_USERNAME=<API Key>`
   - `FOODATASIM_KAFKA_SASL_PASSWORD=<API Secret>`

3. **Run the Docker Container:**

   You can pass these environment variables directly in the `docker run` command or use an `.env` file for better security and manageability.

   **Using Direct Environment Variables:**

   ```bash
   docker run -d \
     --name foodatasim \
     -e FOODATASIM_KAFKA_ENABLED=true \
     -e FOODATASIM_KAFKA_USE_LOCAL=false \
     -e FOODATASIM_KAFKA_BROKER_LIST="pkc-4wxyz.us-west1.gcp.confluent.cloud:9092" \
     -e FOODATASIM_KAFKA_SECURITY_PROTOCOL="SASL_SSL" \
     -e FOODATASIM_KAFKA_SASL_MECHANISM="PLAIN" \
     -e FOODATASIM_KAFKA_SASL_USERNAME="YOUR_API_KEY" \
     -e FOODATASIM_KAFKA_SASL_PASSWORD="YOUR_API_SECRET" \
     your-dockerhub-username/foodatasim:latest
   ```

   **Using an `.env` File:**

   1. **Create a `.env` File:**

      ```bash
      touch .env
      ```

   2. **Add the Following to `.env`:**

      ```env
      FOODATASIM_KAFKA_ENABLED=true
      FOODATASIM_KAFKA_USE_LOCAL=false
      FOODATASIM_KAFKA_BROKER_LIST=pkc-4wxyz.us-west1.gcp.confluent.cloud:9092
      FOODATASIM_KAFKA_SECURITY_PROTOCOL=SASL_SSL
      FOODATASIM_KAFKA_SASL_MECHANISM=PLAIN
      FOODATASIM_KAFKA_SASL_USERNAME=YOUR_API_KEY
      FOODATASIM_KAFKA_SASL_PASSWORD=YOUR_API_SECRET
      ```

   3. **Run the Docker Container with the `.env` File:**

      ```bash
      docker run -d \
        --name foodatasim \
        --env-file .env \
        your-dockerhub-username/foodatasim:latest
      ```

      **Security Note:** Ensure that your `.env` file is secured and not committed to version control systems like Git.

4. **Verify the Container is Running:**

   ```bash
   docker ps
   ```

   You should see an entry similar to:

   ```
   CONTAINER ID   IMAGE                           COMMAND         CREATED          STATUS          PORTS                    NAMES
   abcdef123456   your-dockerhub-username/foodatasim:latest   "./foodatasim"   10 seconds ago   Up 8 seconds    0.0.0.0:9092->9092/tcp   foodatasim
   ```

5. **Check Logs for Confirmation:**

   ```bash
   docker logs -f foodatasim
   ```

   Look for logs indicating that Kafka output is enabled and messages are being produced to Confluent Cloud.

### Running the Docker Container with Custom Configuration

You can also pass additional configuration options or override defaults as needed. Below are some examples:

#### Example 1: Kafka Enabled with Local Kafka

```bash
docker run -d \
  --name foodatasim-local-kafka \
  -e FOODATASIM_KAFKA_ENABLED=true \
  -e FOODATASIM_KAFKA_USE_LOCAL=true \
  -e FOODATASIM_KAFKA_BROKER_LIST="localhost:9092" \
  your-dockerhub-username/foodatasim:latest \
  --config "./examples/config.json"
```

#### Example 2: Kafka Enabled with Confluent Cloud

```bash
docker run -d \
  --name foodatasim-confluent-cloud \
  -e FOODATASIM_KAFKA_ENABLED=true \
  -e FOODATASIM_KAFKA_USE_LOCAL=false \
  -e FOODATASIM_KAFKA_BROKER_LIST="pkc-4wxyz.us-west1.gcp.confluent.cloud:9092" \
  -e FOODATASIM_KAFKA_SECURITY_PROTOCOL="SASL_SSL" \
  -e FOODATASIM_KAFKA_SASL_MECHANISM="PLAIN" \
  -e FOODATASIM_KAFKA_SASL_USERNAME="YOUR_API_KEY" \
  -e FOODATASIM_KAFKA_SASL_PASSWORD="YOUR_API_SECRET" \
  your-dockerhub-username/foodatasim:latest \
  --config "./examples/config.json"
```

**Note:** Replace `"YOUR_API_KEY"` and `"YOUR_API_SECRET"` with your actual Confluent Cloud credentials.

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
