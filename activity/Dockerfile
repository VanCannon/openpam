FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./
# COPY go.sum ./ # Don't copy go.sum yet as it doesn't exist

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
# RUN go mod download

COPY . .

# Build the Go app
RUN go build -o activity-service cmd/server/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/activity-service .

EXPOSE 8083

CMD ["./activity-service"]
