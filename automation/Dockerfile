FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./
# COPY go.sum ./ # Uncomment when we have dependencies
RUN go mod download || true

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./main.go

# Runtime image
FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies: Python, pip, SSH, and Ansible
RUN apt-get update && apt-get install -y \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv \
    openssh-client \
    sshpass \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create a virtual environment for Ansible to isolate dependencies
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# Install Ansible Core and common collections
RUN pip3 install --no-cache-dir ansible-core requests google-auth

# Copy the binary from builder
COPY --from=builder /app/server .

# Expose the service port
EXPOSE 8084

# Run the server
CMD ["./server"]
