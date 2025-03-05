FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the jobctl binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /jobctl ./cmd/jobctl

# Create a minimal image for the final binary
FROM alpine:3.17

# Install bash for better shell experience
RUN apk add --no-cache bash

# Copy the binary from the builder stage
COPY --from=builder /jobctl /usr/local/bin/jobctl

# Set the entrypoint to the jobctl binary
ENTRYPOINT ["jobctl"]

# Default command is help
CMD ["help"] 