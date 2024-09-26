# Stage 1: Build the application
FROM golang:1.20-bullseye AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    librdkafka-dev \
    && rm -rf /var/lib/apt/lists/*

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with CGO enabled and strip debug info
ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w" -o /foodatasim .

# Stage 2: Create the final lightweight image
FROM debian:bullseye-slim

# Install CA certificates and librdkafka runtime library
RUN apt-get update && apt-get install -y \
    ca-certificates \
    librdkafka1 \
    && rm -rf /var/lib/apt/lists/*

# Set the working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /foodatasim .

# Copy necessary data and config files
COPY --from=builder /app/data /root/data
COPY --from=builder /app/examples /root/examples

# Ensure the binary has execution permissions
RUN chmod +x /root/foodatasim

# Set the entrypoint to the application binary
ENTRYPOINT ["./foodatasim"]

# Default command arguments
CMD ["--config", "./examples/config.json"]
