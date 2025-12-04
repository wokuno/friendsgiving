# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy go mod file
COPY go.mod ./
# Download dependencies (if any)
RUN go mod download

# Copy source code
COPY src ./src

# Build the application
RUN go build -o friendsgiving-server ./src

# Run stage
FROM alpine:latest
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/friendsgiving-server .

# Copy static files and data
COPY --from=builder /app/src/static ./static
COPY --from=builder /app/src/data ./data

# Expose the port
EXPOSE 8000

# Run the application
CMD ["./friendsgiving-server"]
