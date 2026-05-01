#!/bin/bash

# Telecom Platform API Gateway Startup Script
# This script starts the platform with Traefik as the API Gateway

set -e

echo "Starting Telecom Platform with API Gateway..."

# Check for required environment variables
if [ -z "$JWT_SECRET" ]; then
    echo "WARNING: JWT_SECRET not set, using default for development only"
    export JWT_SECRET="change-me-in-production-jwt-secret-32-chars-min"
fi

# Create necessary directories
mkdir -p traefik/dynamic

# Stop any existing containers
echo "Stopping existing containers..."
docker-compose down

# Build and start services
echo "Building and starting services with API Gateway..."
docker-compose up --build -d

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 10

# Check service health
echo "Checking service health..."

# Check Traefik
if curl -f http://localhost:8080/ping >/dev/null 2>&1; then
    echo "Traefik: OK"
else
    echo "Traefik: FAILED"
fi

# Check API Server through gateway
if curl -f http://localhost/api/v1/health >/dev/null 2>&1; then
    echo "API Server: OK"
else
    echo "API Server: FAILED"
fi

# Check Charging Engine through gateway
if curl -f http://localhost/v1/health >/dev/null 2>&1; then
    echo "Charging Engine: OK"
else
    echo "Charging Engine: FAILED"
fi

echo ""
echo "API Gateway is running!"
echo ""
echo "Services:"
echo "  - Traefik Dashboard: http://localhost:8080"
echo "  - API Server: https://api.telecom.com/api/v1"
echo "  - Charging Engine: https://api.telecom.com/v1/credit"
echo "  - Carrier Connector: https://api.telecom.com/v1/es2"
echo "  - Packet Gateway: https://api.telecom.com/v1/packet"
echo "  - Web Dashboard: http://localhost:3000"
echo ""
echo "To add api.telecom.com to your hosts file:"
echo "  echo '127.0.0.1 api.telecom.com' | sudo tee -a /etc/hosts"
echo ""
echo "To view logs:"
echo "  docker-compose logs -f traefik"
echo "  docker-compose logs -f api-server"
echo "  docker-compose logs -f charging-engine"
