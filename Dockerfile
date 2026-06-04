# Stage 1: build the binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first (cached layer if unchanged)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build a static binary, stripped, for Linux amd64
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bookmark-service .

# Stage 2: minimal runtime image
FROM alpine:3.20

# CA certificates for HTTPS to outside services (we'll need these later)
RUN apk add --no-cache ca-certificates

# Copy just the binary from the builder stage
COPY --from=builder /bookmark-service /bookmark-service

EXPOSE 8080

ENTRYPOINT ["/bookmark-service"]
