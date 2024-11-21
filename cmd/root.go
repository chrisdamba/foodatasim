package cmd

import (
	"context"
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/chrisdamba/foodatasim/internal/simulator"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "foodatasim",
	Short: "Simulates streaming data for food delivery platforms",
	Long:  `foodatasim is a CLI tool to simulate event streaming data from online food consumer behaviour, restaurant operations, and delivery processes for fictional food delivery services.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := models.LoadConfig(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// initialise database connection
		dbPool, err := initializeDB(&cfg.Database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
			os.Exit(1)
		}
		defer dbPool.Close()

		err = cfg.LoadReviewData("data/restaurant_reviews.tsv")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading review data: %v", err)
		}
		err = cfg.LoadMenuDishData("data/menu_dishes.csv")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading menu dish data: %v", err)
		}
		sim := simulator.NewSimulator(cfg, dbPool)
		sim.Run()
	},
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.foodatasim.yaml)")

	rootCmd.Flags().Int("seed", 42, "Random seed for simulation")
	rootCmd.Flags().String("start-date", time.Now().Format(time.RFC3339), "Start date for simulation")
	rootCmd.Flags().String("end-date", time.Now().AddDate(0, 1, 0).Format(time.RFC3339), "End date for simulation")
	rootCmd.Flags().Int("initial-users", 1000, "Initial number of users")
	rootCmd.Flags().Int("initial-restaurants", 100, "Initial number of restaurants")
	rootCmd.Flags().Int("initial-partners", 50, "Initial number of delivery partners")
	rootCmd.Flags().Float64("user-growth-rate", 0.01, "Annual user growth rate")
	rootCmd.Flags().Float64("partner-growth-rate", 0.02, "Annual partner growth rate")
	rootCmd.Flags().Float64("order-frequency", 0.1, "Average order frequency per user per day")
	rootCmd.Flags().Float64("peak-hour-factor", 1.5, "Factor for increased order frequency during peak hours")
	rootCmd.Flags().Float64("weekend-factor", 1.2, "Factor for increased order frequency during weekends")
	rootCmd.Flags().Float64("traffic-variability", 0.2, "Variability in traffic conditions")
	rootCmd.Flags().Bool("kafka-enabled", false, "Enable Kafka output")
	rootCmd.Flags().String("kafka-broker-list", "localhost:9092", "Kafka broker list")
	rootCmd.Flags().String("output-file", "", "Output file path (if not using Kafka)")
	rootCmd.Flags().Bool("continuous", false, "Run simulation in continuous mode")

	viper.BindPFlags(rootCmd.Flags())
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".foodatasim")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initializeDB(dbConfig *models.DatabaseConfig) (*pgxpool.Pool, error) {
	ctx := context.Background()

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName,
		dbConfig.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing connection string: %v", err)
	}

	poolConfig.MaxConns = 50
	poolConfig.MinConns = 10
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	return pool, nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
