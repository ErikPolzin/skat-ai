# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary with environment variable to allow unsafe package
RUN CGO_ENABLED=0 GOOS=linux ASSUME_NO_MOVING_GC_UNSAFE_RISK_IT_WITH=go1.25 go build -o /server ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /server .

# Expose port 8080
EXPOSE 8080

# Run the server
CMD ["./server"]
