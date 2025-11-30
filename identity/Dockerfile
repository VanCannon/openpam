# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
# COPY go.sum ./
# RUN go mod download

COPY . .

RUN go build -o /identity cmd/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /identity /app/identity

EXPOSE 8082

CMD ["/app/identity"]
