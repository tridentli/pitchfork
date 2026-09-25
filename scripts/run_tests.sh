#!/bin/sh
set -e

DB_HOST=${DB_HOST:-127.0.0.1}
DB_ADMIN_PASS=${DB_ADMIN_PASS:-postgres_password}
CONF_DIR=${PITCHFORK_CONFROOT:-$PWD/test/config}

# Wait for PostgreSQL to be ready if we are in docker (DB_HOST is not localhost)
echo "Waiting for database at $DB_HOST to be ready..."
until pg_isready -h "$DB_HOST" -p 5432 -U postgres; do
  sleep 1
done

echo "Database is ready!"

# Ensure directories exist
mkdir -p "$CONF_DIR"
mkdir -p test/var

# Generate JWT keys if they don't exist
if [ ! -f "$CONF_DIR/jwt.prv" ]; then
    echo "Generating JWT keys..."
    openssl ecparam -name secp521r1 -genkey -noout -out "$CONF_DIR/jwt.prv"
    openssl ec -in "$CONF_DIR/jwt.prv" -pubout -out "$CONF_DIR/jwt.pub"
fi

# Generate pitchfork.conf from template
echo "Generating pitchfork.conf..."
sed -e "s/DB_HOST_PLACEHOLDER/$DB_HOST/" \
    -e "s/DB_ADMIN_PASS_PLACEHOLDER/$DB_ADMIN_PASS/" \
    test/pitchfork.conf.template > "$CONF_DIR/pitchfork.conf"

# Build setup tool
echo "Building setup tool..."
go build -o bin/pfsetup cmd/setup/main/main.go

# Run migrations and setup test DB
echo "Setting up test database..."
PITCHFORK_TOOLNAME=pitchfork PITCHFORK_CONFROOT="$CONF_DIR" ./bin/pfsetup --force-db-destroy setup_test_db

# Run tests
echo "Running tests..."
PITCHFORK_TOOLNAME=pitchfork PITCHFORK_CONFROOT="$CONF_DIR" go test -v ./...
