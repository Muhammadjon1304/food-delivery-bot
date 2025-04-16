# Stage 1: Build the bot
FROM golang:1.23 AS builder
WORKDIR /app

# Copy source files
COPY . .

# Download dependencies
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bot ./cmd

# Stage 2: Minimal runtime image
FROM alpine:latest
WORKDIR /root/

# Install PostgreSQL client
RUN apk add --no-cache postgresql-client

# Copy the built bot
COPY --from=builder /app/bot .

# Set environment variables (override in docker-compose)
ENV TELEGRAM_BOT_TOKEN=""
ENV DATABASE_URL="postgres://user:password@db:5432/mydb?sslmode=disable"

# Expose no ports (since it's a bot)
CMD ["./bot"]
