# Build stage
FROM golang:alpine AS builder
WORKDIR /app

# Copy go mod file
COPY go.mod ./
# No external dependencies, so we skip go mod download

# Copy source code
COPY main.go ./

# Build the application
RUN go build -o friendsgiving-server main.go

# Run stage
FROM alpine:latest
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/friendsgiving-server .

# Copy static files and data
COPY index.html .
COPY menu.json .

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./friendsgiving-server"]
