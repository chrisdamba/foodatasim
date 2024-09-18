FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o /foodatasim .

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /foodatasim .

# Copy necessary data and config files
COPY --from=builder /app/data /root/data
COPY --from=builder /app/examples /root/examples
COPY --from=builder /app/client.properties /root/

# Command to run
CMD ["./foodatasim", "--config", "./examples/config.json"]
