# Build stage
FROM golang:1.23.5-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o atfun

# Final stage
FROM alpine:latest

# Add non root user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/atfun .

# Copy static files
COPY static/ static/

# Use non root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./atfun"]
