package simulator

import (
	"github.com/chrisdamba/foodatasim/internal/models"
	"math"
	"math/rand"
	"time"
)

type DeliveryCluster struct {
	Name           string
	TrafficDensity float64
	SpeedRange     struct {
		Min float64
		Max float64
	}
	BaseSuccessRate float64
	PeakSlowdown    float64
}

type RatingDistribution struct {
	ClusterWeights map[string]float64
	TimeWeights    map[string]float64
	WeatherEffects map[string]float64
}

var (
	DeliveryClusters = map[string]DeliveryCluster{
		"urban_core": {
			Name:           "urban_core",
			TrafficDensity: 1.5,
			SpeedRange: struct {
				Min float64
				Max float64
			}{15, 35},
			BaseSuccessRate: 0.92,
			PeakSlowdown:    0.7,
		},
		"urban_residential": {
			Name:           "urban_residential",
			TrafficDensity: 1.2,
			SpeedRange: struct {
				Min float64
				Max float64
			}{20, 45},
			BaseSuccessRate: 0.95,
			PeakSlowdown:    0.8,
		},
		"suburban": {
			Name:           "suburban",
			TrafficDensity: 0.8,
			SpeedRange: struct {
				Min float64
				Max float64
			}{25, 55},
			BaseSuccessRate: 0.97,
			PeakSlowdown:    0.9,
		},
	}

	RatingDistributions = RatingDistribution{
		ClusterWeights: map[string]float64{
			"urban_core":        1.2,
			"urban_residential": 1.0,
			"suburban":          0.8,
		},
		TimeWeights: map[string]float64{
			"peak_hours":   0.9, // More critical during peak hours
			"normal_hours": 1.0,
			"off_peak":     1.1, // More lenient during off-peak
		},
		WeatherEffects: map[string]float64{
			"rain":   0.9,
			"snow":   0.8,
			"normal": 1.0,
		},
	}
)

func (s *Simulator) generateNormalizedRating(mean, std, min, max float64) float64 {
	// Box-Muller transform for normal distribution
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

	// scale and shift to desired mean and std
	rating := mean + z*std

	// clamp to allowed range
	return math.Max(min, math.Min(max, rating))
}

func (s *Simulator) getLocationCluster(location models.Location) string {
	distanceFromCenter := s.calculateDistance(location, models.Location{
		Lat: s.Config.CityLat,
		Lon: s.Config.CityLon,
	})

	switch {
	case distanceFromCenter <= 2.0:
		return "urban_core"
	case distanceFromCenter <= 5.0:
		return "urban_residential"
	default:
		return "suburban"
	}
}

func (s *Simulator) calculateAdjustedDeliverySpeed(partner *models.DeliveryPartner, currentTime time.Time) float64 {
	cluster := DeliveryClusters[s.getLocationCluster(partner.CurrentLocation)]
	baseSpeed := partner.AvgSpeed

	// time-based adjustments
	timeMultiplier := s.getTimeBasedSpeedMultiplier(currentTime)

	// weather adjustments
	weatherMultiplier := s.getWeatherSpeedMultiplier()

	// traffic density adjustments
	trafficMultiplier := 1.0 / cluster.TrafficDensity

	// experience bonus (up to 20% increase for experienced drivers)
	experienceMultiplier := 1.0 + (partner.Experience * 0.2)

	adjustedSpeed := baseSpeed * timeMultiplier * weatherMultiplier * trafficMultiplier * experienceMultiplier

	// clamp to cluster's speed range
	return math.Max(cluster.SpeedRange.Min, math.Min(cluster.SpeedRange.Max, adjustedSpeed))
}

func (s *Simulator) getTimeBasedSpeedMultiplier(t time.Time) float64 {
	hour := t.Hour()
	weekday := t.Weekday()

	// base multiplier
	multiplier := 1.0

	// peak hour slowdown
	if (hour >= 7 && hour <= 9) || (hour >= 16 && hour <= 18) {
		multiplier *= 0.7 // 30% slower during peak hours
	}

	// late night bonus
	if hour >= 22 || hour <= 4 {
		multiplier *= 1.3 // 30% faster during night
	}

	// weekend adjustments
	if weekday == time.Saturday || weekday == time.Sunday {
		if hour >= 10 && hour <= 20 {
			multiplier *= 0.85 // 15% slower during weekend daytime
		}
	}

	return multiplier
}

func (s *Simulator) getWeatherSpeedMultiplier() float64 {
	// could be expanded to use real weather data or more sophisticated patterns
	if s.isRainyOrColdWeather() {
		return 0.8 // 20% slower in bad weather
	}
	return 1.0
}
