package models

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
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

type Config struct {
	Seed               int       `mapstructure:"seed"`
	StartDate          time.Time `mapstructure:"start_date"`
	EndDate            time.Time `mapstructure:"end_date"`
	InitialUsers       int       `mapstructure:"initial_users"`
	InitialRestaurants int       `mapstructure:"initial_restaurants"`
	InitialPartners    int       `mapstructure:"initial_partners"`
	UserGrowthRate     float64   `mapstructure:"user_growth_rate"`
	PartnerGrowthRate  float64   `mapstructure:"partner_growth_rate"`
	OrderFrequency     float64   `mapstructure:"order_frequency"`
	PeakHourFactor     float64   `mapstructure:"peak_hour_factor"`
	WeekendFactor      float64   `mapstructure:"weekend_factor"`
	TrafficVariability float64   `mapstructure:"traffic_variability"`
	KafkaEnabled       bool      `mapstructure:"kafka_enabled"`
	KafkaBrokerList    string    `mapstructure:"kafka_broker_list"`
	OutputFile         string    `mapstructure:"output_file_path"`
	OutputFolder       string    `mapstructure:"output_folder"`
	Continuous         bool      `mapstructure:"continuous"`
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

	viper.AutomaticEnv() // Read in environment variables that match

	// Set default for start time as the current time if not provided
	viper.SetDefault("start-time", time.Now().Format(time.RFC3339))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
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

	return &config, nil
}

func (cfg *Config) LoadReviewData(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.Read()

	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		liked, _ := strconv.ParseBool(fields[1])
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
