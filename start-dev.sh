#!/bin/bash
# Quick start script for OpenPAM development mode

set -e

echo "🚀 Starting OpenPAM in Development Mode"
echo "========================================="
echo ""

# Check if docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Start services
echo "📦 Starting PostgreSQL and Vault..."
docker-compose up -d postgres vault

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
sleep 5

# Check if migrations need to be run
echo "🔄 Running database migrations..."
cd gateway
go run cmd/migrate/main.go || true

# Copy dev config if .env doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env from .env.dev..."
    cp .env.dev .env
else
    echo "ℹ️  Using existing .env file"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "To start the backend:"
echo "  cd gateway && go run cmd/server/main.go"
echo ""
echo "To start the frontend (in another terminal):"
echo "  cd web && npm install && npm run dev"
echo ""
echo "Then visit: http://localhost:3000"
echo ""
echo "⚠️  Development mode is enabled - auto-login as dev@example.com"
