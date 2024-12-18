package models

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type ReviewData struct {
	Comment string `mapstructure:"comment"`
	Liked   bool   `mapstructure:"liked"`
}

type MenuDish struct {
	Name string `mapstructure:"name"`
}

type CloudStorageConfig struct {
	Provider      string `mapstructure:"provider"`
	BucketName    string `mapstructure:"bucket_name"`
	ContainerName string `mapstructure:"container_name"`
	Region        string `mapstructure:"region"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type Config struct {
	Seed                  int                `mapstructure:"seed"`
	StartDate             time.Time          `mapstructure:"start_date"`
	EndDate               time.Time          `mapstructure:"end_date"`
	InitialUsers          int                `mapstructure:"initial_users"`
	InitialRestaurants    int                `mapstructure:"initial_restaurants"`
	InitialPartners       int                `mapstructure:"initial_partners"`
	UserGrowthRate        float64            `mapstructure:"user_growth_rate"`
	PartnerGrowthRate     float64            `mapstructure:"partner_growth_rate"`
	OrderFrequency        float64            `mapstructure:"order_frequency"`
	PeakHourFactor        float64            `mapstructure:"peak_hour_factor"`
	WeekendFactor         float64            `mapstructure:"weekend_factor"`
	TrafficVariability    float64            `mapstructure:"traffic_variability"`
	KafkaEnabled          bool               `mapstructure:"kafka_enabled"`
	KafkaUseLocal         bool               `mapstructure:"kafka_use_local"`
	KafkaBrokerList       string             `mapstructure:"kafka_broker_list"`
	KafkaSecurityProtocol string             `mapstructure:"kafka_security_protocol"`
	KafkaSaslMechanism    string             `mapstructure:"kafka_sasl_mechanism"`
	KafkaSaslUsername     string             `mapstructure:"kafka_sasl_username"`
	KafkaSaslPassword     string             `mapstructure:"kafka_sasl_password"`
	SessionTimeoutMs      int                `mapstructure:"session_timeout_ms"`
	OutputFormat          string             `mapstructure:"output_format"`
	OutputPath            string             `mapstructure:"output_path"`
	OutputFolder          string             `mapstructure:"output_folder"`
	Continuous            bool               `mapstructure:"continuous"`
	OutputDestination     string             `mapstructure:"output_destination"`
	OutputTypes           []string           `mapstructure:"output_types"` // e.g. ["parquet", "postgres"
	Database              DatabaseConfig     `mapstructure:"database"`
	CloudStorage          CloudStorageConfig `mapstructure:"cloud_storage"`
	// Additional fields
	CityName              string        `mapstructure:"city_name"`
	DefaultCurrency       int           `mapstructure:"default_currency"`
	MinPrepTime           int           `mapstructure:"min_prep_time"`
	MaxPrepTime           int           `mapstructure:"max_prep_time"`
	MinRating             float64       `mapstructure:"min_rating"`
	MaxRating             float64       `mapstructure:"max_rating"`
	MaxInitialRatings     float64       `mapstructure:"max_initial_ratings"`
	MinEfficiency         float64       `mapstructure:"min_efficiency"`
	MaxEfficiency         float64       `mapstructure:"max_efficiency"`
	MinCapacity           int           `mapstructure:"min_capacity"`
	MaxCapacity           int           `mapstructure:"max_capacity"`
	TaxRate               float64       `mapstructure:"tax_rate"`
	ServiceFeePercentage  float64       `mapstructure:"service_fee_percentage"`
	DiscountPercentage    float64       `mapstructure:"discount_percentage"`
	MinOrderForDiscount   float64       `mapstructure:"min_order_for_discount"`
	MaxDiscountAmount     float64       `mapstructure:"max_discount_amount"`
	BaseDeliveryFee       float64       `mapstructure:"base_delivery_fee"`
	FreeDeliveryThreshold float64       `mapstructure:"free_delivery_threshold"`
	SmallOrderThreshold   float64       `mapstructure:"small_order_threshold"`
	SmallOrderFee         float64       `mapstructure:"small_order_fee"`
	RestaurantRatingAlpha float64       `mapstructure:"restaurant_rating_alpha"`
	PartnerRatingAlpha    float64       `mapstructure:"partner_rating_alpha"`
	ReviewGenerationDelay time.Duration `mapstructure:"review_generation_delay"` // How many minutes to wait before leaving a review
	ReviewData            []ReviewData  `mapstructure:"review_data"`
	MenuDishes            []MenuDish    `mapstructure:"menu_dishes"`

	NearLocationThreshold float64 `mapstructure:"near_location_threshold"`
	CityLat               float64 `mapstructure:"city_latitude"`
	CityLon               float64 `mapstructure:"city_longitude"`
	UrbanRadius           float64 `mapstructure:"urban_radius"`
	HotspotRadius         float64 `mapstructure:"hotspot_radius"`
	PartnerMoveSpeed      float64 `mapstructure:"partner_move_speed"`    // km per time unit
	LocationPrecision     float64 `mapstructure:"location_precision"`    // For isAtLocation
	UserBehaviourWindow   int     `mapstructure:"user_behaviour_window"` // Number of orders to consider for adjusting frequency
	RestaurantLoadFactor  float64 `mapstructure:"restaurant_load_factor"`
	EfficiencyAdjustRate  float64 `mapstructure:"efficiency_adjust_rate"`
}

// LoadConfig initializes and reads the configuration using Viper
func LoadConfig(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Default config location
		viper.AddConfigPath("examples")
		viper.SetConfigName("config")
		viper.SetConfigType("json")
	}
	// set the environment variable prefix to avoid conflicts
	viper.SetEnvPrefix("FOODATASIM")

	viper.AutomaticEnv() // read in environment variables that match

	// Replace dots with underscores in config keys when searching for environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Bind environment variables to config keys
	bindEnvVariables()

	// set default for start time as the current time if not provided
	viper.SetDefault("start-time", time.Now().Format(time.RFC3339))

	// read in the config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// it's okay if the config file is not found; we can rely on env vars and defaults
	}

	var config Config
	decoderConfigOption := viper.DecoderConfigOption(func(config *mapstructure.DecoderConfig) {
		config.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			config.DecodeHook,
			mapstructure.StringToTimeHookFunc(time.RFC3339),
		)
	})
	if err := viper.Unmarshal(&config, decoderConfigOption); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}

	// validate cloud storage configuration
	if config.OutputDestination != "local" {
		if err := validateCloudStorageConfig(&config); err != nil {
			return nil, err
		}
	}

	return &config, nil
}

func (cfg *Config) LoadReviewData(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'         // use tab as the delimiter
	reader.LazyQuotes = true    // allow lazy quoting
	reader.FieldsPerRecord = -1 // allow variable number of fields
	reader.Read()               // skip the header line

	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Warning: skipping line due to error: %v\n", err)
			continue
		}
		if len(fields) < 2 {
			fmt.Printf("Warning: skipping incomplete line: %v\n", fields)
			continue
		}
		// handle parsing "liked" field as integer (0 or 1)
		likedInt, err := strconv.Atoi(fields[1])
		if err != nil {
			fmt.Printf("Warning: invalid liked value on line: %v\n", fields)
			continue
		}
		liked := likedInt != 0
		review := ReviewData{
			Comment: fields[0],
			Liked:   liked,
		}
		cfg.ReviewData = append(cfg.ReviewData, review)
	}

	return nil
}

func (cfg *Config) LoadMenuDishData(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Read()

	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dish := MenuDish{
			Name: fields[1],
		}
		cfg.MenuDishes = append(cfg.MenuDishes, dish)
	}

	return nil
}

func validateCloudStorageConfig(config *Config) error {
	switch config.CloudStorage.Provider {
	case "gcs":
		if config.CloudStorage.BucketName == "" {
			return fmt.Errorf("bucket_name is required for GCS")
		}
	case "s3":
		if config.CloudStorage.BucketName == "" {
			return fmt.Errorf("bucket_name is required for S3")
		}
		if config.CloudStorage.Region == "" {
			return fmt.Errorf("region is required for S3")
		}
	case "azure":
		if config.CloudStorage.ContainerName == "" {
			return fmt.Errorf("container_name is required for Azure Blob Storage")
		}
	default:
		return fmt.Errorf("unsupported cloud storage provider: %s", config.CloudStorage.Provider)
	}
	return nil
}

func bindEnvVariables() {
	// list all the keys to bind to environment variables
	keys := []string{
		"seed",
		"start_date",
		"end_date",
		"initial_users",
		"initial_restaurants",
		"initial_partners",
		"user_growth_rate",
		"partner_growth_rate",
		"order_frequency",
		"peak_hour_factor",
		"weekend_factor",
		"traffic_variability",
		"kafka_enabled",
		"kafka_use_local",
		"kafka_broker_list",
		"kafka_security_protocol",
		"kafka_sasl_mechanism",
		"kafka_sasl_username",
		"kafka_sasl_password",
		"session_timeout_ms",
		"output_format",
		"output_path",
		"output_folder",
		"continuous",
		"output_destination",
		"cloud_storage.provider",
		"cloud_storage.bucket_name",
		"cloud_storage.container_name",
		"cloud_storage.region",
		// add other keys as needed
	}

	for _, key := range keys {
		viper.BindEnv(key)
	}
}
