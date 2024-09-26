package simulator

import (
	"fmt"
	"github.com/IBM/sarama"
	"github.com/chrisdamba/foodatasim/internal/models"
	"log"
	"strings"
	"time"
)

type SaramaProducer struct {
	producer sarama.SyncProducer
}

func NewSaramaProducer(config *models.Config) (*SaramaProducer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 5
	saramaConfig.Producer.Retry.Backoff = 100 * time.Millisecond
	saramaConfig.Producer.Return.Successes = true // Must be true for SyncProducer
	saramaConfig.Net.DialTimeout = 30 * time.Second
	saramaConfig.Net.ReadTimeout = 30 * time.Second
	saramaConfig.Net.WriteTimeout = 30 * time.Second

	// session timeout for higher availability (Best practice)
	if config.SessionTimeoutMs > 0 {
		saramaConfig.Consumer.Group.Session.Timeout = time.Duration(config.SessionTimeoutMs) * time.Millisecond
	} else {
		saramaConfig.Consumer.Group.Session.Timeout = 45 * time.Second // default value
	}

	brokerList := strings.Split(config.KafkaBrokerList, ",")

	producer, err := sarama.NewSyncProducer(brokerList, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sarama producer: %w", err)
	}

	log.Printf("Sarama producer created successfully with brokers %v", brokerList)
	return &SaramaProducer{producer: producer}, nil
}

func (s *SaramaProducer) WriteMessage(topic string, msg []byte) error {
	if s.producer == nil {
		return fmt.Errorf("Sarama producer is not initialized")
	}

	_, _, err := s.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(msg),
	})
	if err != nil {
		log.Printf("Failed to send message to topic %s: %v", topic, err)
		return err
	}

	return nil
}

func (s *SaramaProducer) Close() error {
	if s.producer != nil {
		return s.producer.Close()
	}
	return nil
}
