FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway-service cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/gateway-service .
# Assuming config might be needed, but for now just the binary.
# If there's a config.yaml, we should copy it too. But I didn't see one in the root.

EXPOSE 8080

CMD ["./gateway-service"]
