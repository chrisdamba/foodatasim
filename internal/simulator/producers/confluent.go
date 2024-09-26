package simulator

import (
	"fmt"
	"log"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type ConfluentProducer struct {
	producer *kafka.Producer
}

func NewConfluentProducer(configMap kafka.ConfigMap) (*ConfluentProducer, error) {
	producer, err := kafka.NewProducer(&configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create Confluent Kafka producer: %w", err)
	}

	// start a goroutine to handle delivery reports
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Printf("Failed to deliver message to %v: %v\n", ev.TopicPartition, ev.TopicPartition.Error)
				} else {
					log.Printf("Message delivered to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	log.Printf("Confluent Kafka producer created successfully")
	return &ConfluentProducer{producer: producer}, nil
}

// WriteMessage sends a message to the specified topic using Confluent's Kafka client
func (c *ConfluentProducer) WriteMessage(topic string, msg []byte) error {
	if c.producer == nil {
		return fmt.Errorf("Confluent Kafka producer is not initialized")
	}

	err := c.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg,
	}, nil)
	if err != nil {
		log.Printf("Failed to produce message to topic %s: %v", topic, err)
		return err
	}

	// optionally, wait for message delivery
	c.producer.Flush(1000)
	return nil
}

func (c *ConfluentProducer) Close() error {
	if c.producer != nil {
		c.producer.Close()
	}
	return nil
}
