foodatasim
=======

FoodataSim (Food Data Simulator) is a program that generates synthetic event data for food delivery platforms. Written in Go, it's designed to simulate user behaviour, restaurant operations, and delivery processes for a fictional food delivery service. The generated data looks like real usage patterns but is entirely synthetic. You can configure the program to create data for various scenarios: from a small number of users over a few hours to a large user base over several years. The data can be written to files or streamed to Apache Kafka.

Use the synthetic data for product development, testing, demos, performance evaluation, training, or any situation where a stream of realistic food delivery data is beneficial.

Disclaimer: Note that this data should not be used for actual machine learning algorithm research or to draw conclusions about real-world user behaviour/analysis.

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

Config File
===========
The config file is a JSON file with key-value pairs. Here's an explanation of some key parameters:

* `seed` For the pseudo-random number generator
* `start_date` Start date for data generation (ISO8601 format)
* `end_date` End date for data generation (ISO8601 format)
* `n_users` Initial number of users
* `n_restaurants` Number of restaurants
* `n_delivery_partners` Number of delivery partners
* `growth_rate` Annual growth rate for users
* `order_frequency` Average number of orders per user per month
* `peak_hour_factor` Factor to increase order frequency during peak hours
* `weekend_factor` Factor to adjust order frequency on weekends

The config file also specifies distributions for various parameters such as order value, delivery time, and user ratings.

Usage
=====

To build the executable, run:

    $ go build -o bin/foodata

The program accepts several command-line options:

    $ bin/foodata --help
        -c, --config <arg>             config file
        -o, --output <arg>             output file (default: stdout)
        -f, --format <arg>             output format (json, csv, parquet)
        -k, --kafka-topic <arg>        Kafka topic (if using Kafka output)
        --kafka-broker-list <arg>      Kafka broker list
        -s, --start-date <arg>         start date for data generation
        -e, --end-date <arg>           end date for data generation
        -n, --n-users <arg>            initial number of users
        -g, --growth-rate <arg>        annual user growth rate
        --seed <arg>                   random seed
        --continuous                   run in continuous mode
        --help                         show this help message

Example for generating about 1 million events (10,000 users for a month, growing at 5% annually):

    $ bin/foodata -c "examples/config.json" -s "2023-01-01" -e "2023-02-01" -n 10000 -g 0.05 -o data/synthetic.json

Building large datasets in parallel
===================================
For generating large datasets quickly, you can run multiple instances of Foodatasim simultaneously:

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
