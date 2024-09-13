package cloudwriter

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Writer struct {
	client     *s3.Client
	bucket     string
	objectPath string
	buffer     bytes.Buffer
}

type S3WriterFactory struct {
	client *s3.Client
}

func NewS3WriterFactory(region string) (*S3WriterFactory, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	return &S3WriterFactory{client: client}, nil
}

func (f *S3WriterFactory) NewWriter(bucket, objectPath string) (CloudWriter, error) {
	return &S3Writer{
		client:     f.client,
		bucket:     bucket,
		objectPath: objectPath,
	}, nil
}

func (w *S3Writer) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *S3Writer) Close() error {
	ctx := context.Background()
	_, err := w.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(w.bucket),
		Key:    aws.String(w.objectPath),
		Body:   bytes.NewReader(w.buffer.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("unable to upload file to S3: %v", err)
	}
	return nil
}
