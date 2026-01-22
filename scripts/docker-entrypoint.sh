#!/bin/bash
# =============================================================================
# Docker Entrypoint Script for TR-Only Shipping Aggregation Backend
# =============================================================================

set -e

# Default command
CMD=${1:-api}

# Run migrations if AUTO_MIGRATE is set
if [ "$AUTO_MIGRATE" = "true" ]; then
    echo "Running database migrations..."
    /app/migrate -direction up
fi

# Execute the appropriate binary
case "$CMD" in
    api)
        echo "Starting API server..."
        exec /app/api
        ;;
    worker)
        echo "Starting background worker..."
        exec /app/worker
        ;;
    migrate)
        echo "Running migrations..."
        shift
        exec /app/migrate "$@"
        ;;
    *)
        echo "Unknown command: $CMD"
        echo "Available commands: api, worker, migrate"
        exit 1
        ;;
esac
