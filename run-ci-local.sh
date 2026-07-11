#!/bin/bash
set -e

echo "========================================"
echo "Running full CI locally"
echo "========================================"

# Get absolute path to script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Stop the backgrounded API and containers on exit. Killing API_PID here
# matters because `set -e` can abort after the API is started (e.g. a failing
# e2e run) and skip the explicit kill, leaking a server on port 8080.
cleanup() {
    if [ -n "${API_PID:-}" ]; then
        kill "$API_PID" 2>/dev/null || true
    fi
    log_info "Stopping containers..."
    docker compose -f /tmp/circl-ci-compose.yml down 2>/dev/null || true
}
trap cleanup EXIT

# ============================================
# 1. FRONTEND (lint + unit tests)
# ============================================
echo ""
echo "========================================"
echo "1. Frontend: Lint + Unit Tests"
echo "========================================"

cd "${SCRIPT_DIR}/frontend"

if [ ! -f "package.json" ]; then
    log_warn "No frontend found, skipping..."
else
    log_info "Installing frontend dependencies..."
    npm ci --silent

    log_info "Running lint..."
    npm run lint --if-present

    log_info "Running unit tests..."
    npm test --if-present -- --watch=false
fi

# ============================================
# 2. BACKEND (vet + unit tests)
# ============================================
echo ""
echo "========================================"
echo "2. Backend: Vet + Unit Tests"
echo "========================================"

cd "${SCRIPT_DIR}/backend"

if [ ! -f "go.mod" ]; then
    log_warn "No backend found, skipping..."
else
    log_info "Running go vet..."
    go vet ./...

    log_info "Running unit tests..."
    go test -v ./...
fi

# ============================================
# 3. BACKEND INTEGRATION TESTS
# ============================================
echo ""
echo "========================================"
echo "3. Backend: Integration Tests"
echo "========================================"

if [ ! -f "go.mod" ]; then
    log_warn "No backend found, skipping..."
else
    # Check if docker is available
    if ! command_exists docker; then
        log_error "Docker is not installed. Cannot run integration tests."
        exit 1
    fi

    log_info "Starting PostgreSQL and Redis..."
    
    # Create docker-compose file
    cat > /tmp/circl-ci-compose.yml << 'EOF'
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: circl_user
      POSTGRES_PASSWORD: circl_password
      POSTGRES_DB: circl_db
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U circl_user -d circl_db"]
      interval: 10s
      timeout: 5s
      retries: 5
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD-SHELL", "redis-cli ping"]
      interval: 10s
      timeout: 5s
      retries: 5
EOF

    # Check if services are already running locally (not via docker)
    if pg_isready -U circl_user -d circl_db -h localhost -p 5432 >/dev/null 2>&1; then
        log_warn "PostgreSQL already running locally, reusing..."
    else
        # Try to start with docker (will fail if ports in use but that's ok)
        docker compose -f /tmp/circl-ci-compose.yml up -d 2>/dev/null || true
    fi

    # Wait for services to be healthy
    log_info "Waiting for PostgreSQL..."
    for i in $(seq 1 30); do
        if pg_isready -U circl_user -d circl_db -h localhost -p 5432 >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done

    log_info "Waiting for Redis..."
    for i in $(seq 1 30); do
        if redis-cli -h localhost -p 6379 ping >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done

    log_info "Building API..."
    go build -o /tmp/circl-api ./cmd/api

    log_info "Starting API server (runs migrations on boot) for health check..."
    DATABASE_URL="postgres://circl_user:circl_password@localhost:5432/circl_db?sslmode=disable" \
    REDIS_URL="redis://localhost:6379/0" \
    ASYNQ_REDIS_URL="redis://localhost:6379/1" \
    JWT_SECRET="integration-test-jwt-secret-32chars!!" \
    STORAGE_PROVIDER="local" \
    CORS_ALLOWED_ORIGINS="http://localhost" \
    /tmp/circl-api &
    API_PID=$!
    
    timeout 30 bash -c 'until curl -sf http://localhost:8080/health; do sleep 1; done' || {
        log_error "API failed to start"
        kill $API_PID 2>/dev/null || true
        exit 1
    }
    
    log_info "API is healthy, stopping..."
    kill $API_PID 2>/dev/null || true
    wait $API_PID 2>/dev/null || true

    log_info "Running integration tests..."
    TEST_DATABASE_URL="postgres://circl_user:circl_password@localhost:5432/circl_db?sslmode=disable" \
    TEST_REDIS_URL="redis://localhost:6379/2" \
    go test -v ./...
fi

# ============================================
# 4. E2E TESTS
# ============================================
echo ""
echo "========================================"
echo "4. Frontend: E2E Tests"
echo "========================================"

cd "${SCRIPT_DIR}/frontend"

if [ ! -f "package.json" ]; then
    log_warn "No frontend found, skipping..."
elif [ ! -f "playwright.config.ts" ] && [ ! -f "playwright.config.js" ]; then
    log_warn "No Playwright config found, skipping e2e..."
else
    # Check if services are already running (don't try to start them)
    log_info "Waiting for PostgreSQL..."
    timeout 30 bash -c 'until pg_isready -U circl_user -d circl_db -h localhost -p 5432 2>/dev/null; do sleep 1; done' || true
    log_info "Waiting for Redis..."
    timeout 30 bash -c 'until redis-cli -h localhost -p 6379 ping 2>/dev/null | grep -q PONG; do sleep 1; done' || true

    log_info "Building and starting backend..."
cd "${SCRIPT_DIR}/backend"
    
    go build -o /tmp/circl-api ./cmd/api

    DATABASE_URL="postgres://circl_user:circl_password@localhost:5432/circl_db?sslmode=disable" \
    REDIS_URL="redis://localhost:6379/0" \
    ASYNQ_REDIS_URL="redis://localhost:6379/1" \
    JWT_SECRET="e2e-test-jwt-secret-minimum-32characters!" \
    STORAGE_PROVIDER="local" \
    CORS_ALLOWED_ORIGINS="http://localhost:3000" \
    LOGIN_IP_LIMIT=1000 \
    REGISTER_IP_LIMIT=1000 \
    GLOBAL_IP_LIMIT=0 \
    TEST_ENDPOINTS_ENABLED=true /tmp/circl-api &
    API_PID=$!

    timeout 30 bash -c 'until curl -sf http://localhost:8080/health; do sleep 1; done' || {
        log_error "API failed to start"
        kill $API_PID 2>/dev/null || true
        exit 1
    }

    cd "${SCRIPT_DIR}/frontend"
    
    log_info "Installing Playwright browsers..."
    npx playwright install --with-deps chromium 2>/dev/null || true

    log_info "Running E2E tests..."
    CI=true \
    NEXTAUTH_URL='http://localhost:3000' \
    NEXTAUTH_SECRET='e2e-test-secret-for-playwright-32chars!' \
    BACKEND_URL='http://localhost:8080' \
    NEXT_PUBLIC_API_URL='http://localhost:8080' \
    npm run test:e2e

    log_info "Stopping API..."
    kill $API_PID 2>/dev/null || true
    wait $API_PID 2>/dev/null || true
fi

# ============================================
# DONE
# ============================================
echo ""
echo "========================================"
echo -e "${GREEN}All CI tests passed!${NC}"
echo "========================================"