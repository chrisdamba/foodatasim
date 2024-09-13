package cloudwriter

type CloudWriter interface {
	Write(data []byte) (int, error)
	Close() error
}

type CloudWriterFactory interface {
	NewWriter(bucket, objectPath string) (CloudWriter, error)
}
