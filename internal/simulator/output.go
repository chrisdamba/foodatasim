package simulator

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/chrisdamba/foodatasim/internal/cloudwriter"
	"github.com/chrisdamba/foodatasim/internal/models"
	simulator "github.com/chrisdamba/foodatasim/internal/simulator/producers"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/source"
	"github.com/xitongsys/parquet-go/writer"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type OutputDestination interface {
	WriteMessage(topic string, msg []byte) error
	Close() error
}

type CSVOutput struct {
	basePath string
	folder   string
	files    map[string]*csv.Writer
	headers  map[string][]string
}

type ParquetOutput struct {
	basePath           string
	folder             string
	mu                 sync.Mutex
	writers            map[string]*writer.ParquetWriter
	writerMutexes      map[string]*sync.Mutex
	files              map[string]source.ParquetFile
	cloudWriterFactory cloudwriter.CloudWriterFactory
	cloudBucketName    string
}

type ConsoleOutput struct{}

func (c *ConsoleOutput) Close() error {
	//TODO implement me
	panic("implement me")
}

type KafkaOutput struct {
	producer sarama.SyncProducer
}

type JSONOutput struct {
	basePath string
	folder   string
	files    map[string]*os.File
}

type CloudParquetFile struct {
	cloudWriter cloudwriter.CloudWriter
	offset      int64
}

func (c *CloudParquetFile) Open(name string) (source.ParquetFile, error) {
	// for cloud storage, we don't typically "open" files in the same way as local files.
	// instead, we can return the current instance, as it's already set up for writing.
	return c, nil
}

func (c *CloudParquetFile) Create(name string) (source.ParquetFile, error) {
	// similar to Open, for cloud storage, we don't typically "create" files.
	// the file (or object) is implicitly created when we start writing.
	// we can return the current instance, ready for writing.
	return c, nil
}

func NewCSVOutput(basePath, folder string) *CSVOutput {
	return &CSVOutput{
		basePath: basePath,
		folder:   folder,
		files:    make(map[string]*csv.Writer),
		headers:  make(map[string][]string),
	}
}

func NewJSONOutput(basePath, folder string) *JSONOutput {
	return &JSONOutput{
		basePath: basePath,
		folder:   folder,
		files:    make(map[string]*os.File),
	}
}

func NewParquetOutput(config *models.Config) (*ParquetOutput, error) {
	p := &ParquetOutput{
		basePath:      config.OutputPath,
		folder:        config.OutputFolder,
		writers:       make(map[string]*writer.ParquetWriter),
		writerMutexes: make(map[string]*sync.Mutex),
		files:         make(map[string]source.ParquetFile),
	}

	if config.OutputDestination != "local" {
		var factory cloudwriter.CloudWriterFactory
		var err error

		switch config.CloudStorage.Provider {
		case "gcs":
		case "s3":
			factory, err = cloudwriter.NewS3WriterFactory(config.CloudStorage.Region)
		case "azure":
		default:
			return nil, fmt.Errorf("unsupported cloud storage provider: %s", config.CloudStorage.Provider)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create cloud writer factory: %w", err)
		}

		p.cloudWriterFactory = factory
		p.cloudBucketName = config.CloudStorage.BucketName
	}

	// clean up existing .parquet files
	p.cleanup()

	return p, nil
}

func NewCloudParquetFile(cloudWriter cloudwriter.CloudWriter) *CloudParquetFile {
	return &CloudParquetFile{
		cloudWriter: cloudWriter,
		offset:      0,
	}
}

func (c *CloudParquetFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		c.offset = offset
	case io.SeekCurrent:
		c.offset += offset
	case io.SeekEnd:
		// this might not be applicable for cloud storage
		return 0, fmt.Errorf("seek from end not supported for cloud storage")
	}
	return c.offset, nil
}

func (c *CloudParquetFile) Read(p []byte) (n int, err error) {
	// this might not be applicable for cloud storage
	return 0, fmt.Errorf("read not supported for cloud storage")
}

func (c *CloudParquetFile) Write(p []byte) (n int, err error) {
	return c.cloudWriter.Write(p)
}

func (c *CloudParquetFile) Close() error {
	return c.cloudWriter.Close()
}

func (c *CSVOutput) WriteMessage(topic string, msg []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(msg, &event); err != nil {
		return err
	}

	timestamp, ok := event["timestamp"].(float64)
	if !ok {
		return fmt.Errorf("invalid timestamp")
	}

	eventTime := time.Unix(int64(timestamp), 0)
	year, month, day := eventTime.Date()
	hour := eventTime.Hour()

	partitionPath := fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", year, month, day, hour)
	fullPath := filepath.Join(c.basePath, c.folder, topic, partitionPath)

	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}

	fileKey := fmt.Sprintf("%s_%s", topic, partitionPath)
	csvWriter, ok := c.files[fileKey]
	if !ok {
		file, err := os.Create(filepath.Join(fullPath, "data.csv"))
		if err != nil {
			return err
		}
		csvWriter = csv.NewWriter(file)
		c.files[fileKey] = csvWriter

		// Write headers if this is a new file
		headers := c.getHeaders(event)
		if err := csvWriter.Write(headers); err != nil {
			return err
		}
		c.headers[fileKey] = headers
	}

	// Write the event data
	row := make([]string, len(c.headers[fileKey]))
	for i, header := range c.headers[fileKey] {
		value, ok := event[header]
		if !ok {
			row[i] = ""
		} else {
			row[i] = fmt.Sprintf("%v", value)
		}
	}

	if err := csvWriter.Write(row); err != nil {
		return err
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func (c *CSVOutput) getHeaders(event map[string]interface{}) []string {
	var headers []string
	for key := range event {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func (c *CSVOutput) Close() error {
	for _, csvWriter := range c.files {
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (j *JSONOutput) WriteMessage(topic string, msg []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(msg, &event); err != nil {
		return err
	}

	timestamp, ok := event["timestamp"].(float64)
	if !ok {
		return fmt.Errorf("invalid timestamp")
	}

	eventTime := time.Unix(int64(timestamp), 0)
	year, month, day := eventTime.Date()
	hour := eventTime.Hour()

	partitionPath := fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", year, month, day, hour)
	fullPath := filepath.Join(j.basePath, j.folder, topic, partitionPath)

	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}

	fileKey := fmt.Sprintf("%s_%s", topic, partitionPath)
	file, ok := j.files[fileKey]
	if !ok {
		var err error
		file, err = os.Create(filepath.Join(fullPath, "data.json"))
		if err != nil {
			return err
		}
		j.files[fileKey] = file
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := file.Write(jsonData); err != nil {
		return err
	}
	_, err = file.WriteString("\n")
	return err
}

func (j *JSONOutput) Close() error {
	for _, file := range j.files {
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *ParquetOutput) WriteMessage(topic string, msg []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(msg, &event); err != nil {
		return err
	}

	timestamp, ok := event["timestamp"].(float64)
	if !ok {
		return fmt.Errorf("invalid timestamp")
	}

	eventTime := time.Unix(int64(timestamp), 0)
	year, month, day := eventTime.Date()
	hour := eventTime.Hour()

	partitionPath := fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d", year, month, day, hour)
	fullPath := filepath.Join(p.basePath, p.folder, topic, partitionPath)

	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}

	writerKey := fmt.Sprintf("%s_%s", topic, partitionPath)
	p.mu.Lock()
	pw, ok := p.writers[writerKey]
	if !ok {
		// create new writer if it doesn't exist
		var err error
		pw, err = p.createNewWriter(writerKey, fullPath, topic)
		if err != nil {
			p.mu.Unlock()
			return fmt.Errorf("failed to create new writer: %w", err)
		}
		p.writers[writerKey] = pw
		p.writerMutexes[writerKey] = &sync.Mutex{}
	}
	writerMutex := p.writerMutexes[writerKey]
	p.mu.Unlock()

	writerMutex.Lock()
	defer writerMutex.Unlock()

	if pw == nil {
		return fmt.Errorf("ParquetWriter is nil for key: %s", writerKey)
	}

	if !ok {
		filePath := filepath.Join(fullPath, "data.parquet")
		fw, err := local.NewLocalFileWriter(filePath)
		if err != nil {
			return err
		}

		sc, err := GetSchema(topic)
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}

		pw, err = writer.NewParquetWriter(fw, nil, 4)
		if err != nil {
			return err
		}
		pw.SchemaHandler = sc

		p.mu.Lock() // lock before writing to the map
		p.writers[writerKey] = pw
		p.mu.Unlock() // unlock immediately after
	}

	if err := pw.Write(event); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil

}

func (p *ParquetOutput) cleanup() {
	fullPath := filepath.Join(p.basePath, p.folder)
	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".parquet" {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error cleaning up Parquet files: %v", err)
	}
}

func (p *ParquetOutput) createNewWriter(writerKey, fullPath, topic string) (*writer.ParquetWriter, error) {
	var fw source.ParquetFile
	var err error
	if p.cloudWriterFactory != nil {
		objectPath := filepath.Join(p.folder, topic, fullPath, "data.parquet")
		cloudWriter, err := p.cloudWriterFactory.NewWriter(p.cloudBucketName, objectPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create cloud file writer: %w", err)
		}
		fw = NewCloudParquetFile(cloudWriter)
	} else {
		filePath := filepath.Join(fullPath, "data.parquet")
		fw, err = local.NewLocalFileWriter(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create local file writer: %w", err)
		}
	}

	sc, err := GetSchema(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	pw, err := writer.NewParquetWriter(fw, nil, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to create ParquetWriter: %w", err)
	}
	pw.SchemaHandler = sc

	p.writers[writerKey] = pw
	p.writerMutexes[writerKey] = &sync.Mutex{}
	p.files[writerKey] = fw

	return pw, nil
}

func (p *ParquetOutput) generateSchema(event map[string]interface{}) string {
	sc := make(map[string]interface{})
	for key, value := range event {
		if parquetType := p.getParquetType(value); parquetType != "" {
			sc[key] = parquetType
		}
	}
	schemaJSON, _ := json.Marshal(sc)
	return string(schemaJSON)
}

func (p *ParquetOutput) getParquetType(value interface{}) interface{} {
	switch v := value.(type) {
	case int, int32, int64:
		return "INT64"
	case float32, float64:
		return "DOUBLE"
	case bool:
		return "BOOLEAN"
	case string:
		return "UTF8"
	case []interface{}:
		if len(v) > 0 {
			return fmt.Sprintf("LIST<%s>", p.getParquetType(v[0]))
		}
		return "LIST<UTF8>" // default to string for empty arrays
	case map[string]interface{}:
		nestedSchema := make(map[string]interface{})
		for nestedKey, nestedValue := range v {
			if nestedType := p.getParquetType(nestedValue); nestedType != "" {
				nestedSchema[nestedKey] = nestedType
			}
		}
		if len(nestedSchema) == 0 {
			return "" // return empty string for empty nested objects
		}
		nestedJSON, _ := json.Marshal(nestedSchema)
		return fmt.Sprintf("STRUCT<%s>", string(nestedJSON))
	default:
		return "UTF8" // default to string for unknown types
	}
}

func (p *ParquetOutput) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for key, pw := range p.writers {
		if pw == nil {
			log.Printf("Warning: Nil writer found for key: %s", key)
			continue
		}
		if mutex, ok := p.writerMutexes[key]; ok {
			mutex.Lock()
			if err := pw.WriteStop(); err != nil {
				lastErr = err
				log.Printf("Error closing writer for key %s: %v", key, err)
			}
			if f, ok := p.files[key]; ok {
				if err := f.Close(); err != nil {
					lastErr = err
					log.Printf("Error closing file for key %s: %v", key, err)
				}
			}
			mutex.Unlock()
		}
	}
	return lastErr
}

func (k *KafkaOutput) WriteMessage(topic string, msg []byte) error {
	if k.producer == nil {
		return fmt.Errorf("local Kafka producer is closed")
	}
	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(msg),
	})
	return err
}

func (c *ConsoleOutput) WriteMessage(topic string, msg []byte) error {
	// Create a formatted string that includes the topic
	output := fmt.Sprintf("[%s] %s\n", topic, string(msg))

	// Write the formatted string to stdout
	_, err := os.Stdout.Write([]byte(output))
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %w", err)
	}

	// Try to sync, but don't return an error if it fails
	_ = os.Stdout.Sync()

	return nil
}

func (s *Simulator) determineOutputDestination() OutputDestination {
	if s.Config.KafkaEnabled {
		if s.Config.KafkaUseLocal {
			// use Sarama for local Kafka
			saramaProducer, err := simulator.NewSaramaProducer(s.Config)
			if err != nil {
				log.Fatalf("Failed to create Sarama producer: %v", err)
			}
			return saramaProducer
		} else {
			// use Confluent's Kafka client for Confluent Cloud
			confluentConfig := kafka.ConfigMap{
				"bootstrap.servers":       s.Config.KafkaBrokerList,
				"security.protocol":       s.Config.KafkaSecurityProtocol,
				"sasl.mechanisms":         s.Config.KafkaSaslMechanism,
				"sasl.username":           s.Config.KafkaSaslUsername,
				"sasl.password":           s.Config.KafkaSaslPassword,
				"session.timeout.ms":      s.Config.SessionTimeoutMs,
				"linger.ms":               10,
				"batch.num.messages":      100,
				"compression.type":        "snappy",
				"message.timeout.ms":      300000, // 5 minutes
				"enable.idempotence":      true,
				"acks":                    "all",
				"retry.backoff.ms":        100,
				"socket.keepalive.enable": true,
			}

			confluentProducer, err := simulator.NewConfluentProducer(confluentConfig)
			if err != nil {
				log.Fatalf("Failed to create Confluent Kafka producer: %v", err)
			}
			return confluentProducer
		}
	} else if s.Config.OutputPath != "" {
		switch s.Config.OutputFormat {
		case "parquet":
			output, err := NewParquetOutput(s.Config)
			if err != nil {
				log.Fatalf("Failed to create Parquet output: %s", err)
			}
			return output
		case "json":
			return NewJSONOutput(s.Config.OutputPath, s.Config.OutputFolder)
		case "csv":
			return NewCSVOutput(s.Config.OutputPath, s.Config.OutputFolder)
		default:
			log.Fatalf("Unsupported output format: %s", s.Config.OutputFormat)
		}
	}
	return &ConsoleOutput{}
}
