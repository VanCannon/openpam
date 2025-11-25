FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scheduling-agent ./cmd/scheduling-agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/scheduling-agent .
COPY --from=builder /app/config.yaml .

EXPOSE 8081

CMD ["./scheduling-agent"]
