package cmd

import (
	"fmt"
	"github.com/chrisdamba/foodatasim/internal/models"
	"github.com/chrisdamba/foodatasim/internal/simulator"
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

		sim := simulator.NewSimulator(cfg)
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
