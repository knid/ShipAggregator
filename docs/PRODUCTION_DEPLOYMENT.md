# Production Deployment Guide

This guide covers deploying the TR-Only Shipping Aggregation Backend to production environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Infrastructure Requirements](#infrastructure-requirements)
3. [Deployment Options](#deployment-options)
4. [Environment Configuration](#environment-configuration)
5. [Database Setup](#database-setup)
6. [Docker Deployment](#docker-deployment)
7. [Kubernetes Deployment](#kubernetes-deployment)
8. [CI/CD Pipeline](#cicd-pipeline)
9. [Monitoring & Observability](#monitoring--observability)
10. [Security Checklist](#security-checklist)
11. [Scaling Considerations](#scaling-considerations)
12. [Backup & Recovery](#backup--recovery)

---

## Prerequisites

### Required Tools

- Docker 24.0+ and Docker Compose v2
- Go 1.22+ (for local builds)
- PostgreSQL client tools (`psql`)
- Redis CLI (optional, for debugging)

### Required Services

- PostgreSQL 15+
- Redis 7+
- S3-compatible storage (AWS S3, MinIO, or similar)
- Container registry (Docker Hub, ECR, GCR, etc.)

---

## Infrastructure Requirements

### Minimum Production Specs

| Component | CPU | Memory | Storage | Notes |
|-----------|-----|--------|---------|-------|
| API Server | 2 vCPU | 2 GB | 10 GB | Scale horizontally |
| Worker | 2 vCPU | 2 GB | 10 GB | Scale based on queue depth |
| PostgreSQL | 4 vCPU | 8 GB | 100 GB SSD | Use managed service recommended |
| Redis | 2 vCPU | 4 GB | 10 GB | Use managed service recommended |

### Recommended Production Specs

| Component | CPU | Memory | Storage | Replicas |
|-----------|-----|--------|---------|----------|
| API Server | 4 vCPU | 4 GB | 20 GB | 3+ |
| Worker | 4 vCPU | 4 GB | 20 GB | 2+ |
| PostgreSQL | 8 vCPU | 32 GB | 500 GB SSD | Primary + Read Replica |
| Redis | 4 vCPU | 8 GB | 20 GB | Cluster mode |

---

## Deployment Options

### Option 1: Docker Compose (Single Server)

Best for: Small deployments, staging environments

### Option 2: Kubernetes

Best for: Production, high availability, auto-scaling

### Option 3: Managed Container Services

Best for: AWS ECS, Google Cloud Run, Azure Container Apps

---

## Environment Configuration

### Generate Secure Secrets

```bash
# Generate 32-byte encryption key for AES-256-GCM
openssl rand -hex 32

# Generate JWT secret
openssl rand -base64 64

# Generate webhook signing secret
openssl rand -base64 32
```

### Production Environment Variables

Create a `.env.production` file (never commit to git):

```bash
# =============================================================================
# Application
# =============================================================================
APP_ENV=production
APP_NAME=shipping-aggregator
APP_VERSION=1.0.0
APP_DEBUG=false
LOG_LEVEL=info
LOG_FORMAT=json

# =============================================================================
# HTTP Server
# =============================================================================
API_HOST=0.0.0.0
API_PORT=8080
API_READ_TIMEOUT=30s
API_WRITE_TIMEOUT=30s
API_IDLE_TIMEOUT=120s
API_SHUTDOWN_TIMEOUT=30s

# =============================================================================
# Database (PostgreSQL) - Use managed service in production
# =============================================================================
DATABASE_URL=postgres://user:password@your-db-host:5432/shipping?sslmode=require
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=5m
DB_CONN_MAX_IDLE_TIME=5m

# =============================================================================
# Redis - Use managed service in production
# =============================================================================
REDIS_URL=redis://:password@your-redis-host:6379/0
REDIS_MAX_RETRIES=3
REDIS_POOL_SIZE=20

# =============================================================================
# S3 Storage (AWS S3 or compatible)
# =============================================================================
# For AWS S3:
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
AWS_REGION=eu-west-1
S3_BUCKET_DOCUMENTS=your-shipping-documents
S3_BUCKET_LABELS=your-shipping-labels

# For MinIO/S3-compatible:
MINIO_ENDPOINT=your-minio-endpoint:9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_USE_SSL=true
MINIO_BUCKET_DOCUMENTS=shipping-documents
MINIO_BUCKET_LABELS=shipping-labels

# =============================================================================
# Security & Encryption (REQUIRED - Generate unique values!)
# =============================================================================
ENCRYPTION_KEY=<your-32-byte-hex-key>
JWT_SECRET=<your-jwt-secret>
API_KEY_PREFIX=sk_live_

# =============================================================================
# Rate Limiting
# =============================================================================
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_SECOND=100
RATE_LIMIT_BURST=200

# =============================================================================
# Webhooks
# =============================================================================
WEBHOOK_TIMEOUT=30s
WEBHOOK_MAX_RETRIES=7
WEBHOOK_RETRY_BACKOFF=exponential
WEBHOOK_SIGNING_SECRET=<your-webhook-signing-secret>

# =============================================================================
# Background Jobs
# =============================================================================
ASYNQ_CONCURRENCY=20
ASYNQ_QUEUE_CRITICAL=6
ASYNQ_QUEUE_DEFAULT=3
ASYNQ_QUEUE_LOW=1

# =============================================================================
# Observability
# =============================================================================
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://your-otel-collector:4317
OTEL_SERVICE_NAME=shipping-aggregator
METRICS_ENABLED=true
METRICS_PORT=9090
METRICS_PATH=/metrics

# =============================================================================
# Payment Gateway (Iyzico Production)
# =============================================================================
IYZICO_API_KEY=your-production-api-key
IYZICO_SECRET_KEY=your-production-secret-key
IYZICO_BASE_URL=https://api.iyzipay.com
IYZICO_CALLBACK_URL=https://your-domain.com/api/v1/integrations/iyzico/callback

# =============================================================================
# CORS
# =============================================================================
CORS_ENABLED=true
CORS_ALLOWED_ORIGINS=https://your-frontend-domain.com
CORS_ALLOWED_METHODS=GET,POST,PUT,PATCH,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Authorization,Content-Type,X-Idempotency-Key,X-Trace-ID
CORS_EXPOSED_HEADERS=X-Trace-ID,X-Request-ID,X-Idempotency-Replayed
CORS_MAX_AGE=86400

# =============================================================================
# Data Retention (KVKK Compliance)
# =============================================================================
RETENTION_AUDIT_LOGS_DAYS=365
RETENTION_PROVIDER_LOGS_DAYS=30
RETENTION_TRACKING_EVENTS_DAYS=90
RETENTION_DELETED_DATA_DAYS=30
```

---

## Database Setup

### 1. Create Production Database

```bash
# Connect to PostgreSQL
psql -h your-db-host -U postgres

# Create database and user
CREATE USER shipping WITH PASSWORD 'your-secure-password';
CREATE DATABASE shipping OWNER shipping;
GRANT ALL PRIVILEGES ON DATABASE shipping TO shipping;

# Enable required extensions
\c shipping
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
```

### 2. Run Migrations

```bash
# Using the migration binary
./bin/shipping-migrate -direction up

# Or using Docker
docker run --rm \
  -e DATABASE_URL="postgres://user:pass@host:5432/shipping?sslmode=require" \
  your-registry/shipping-aggregator:latest-migrate \
  -direction up
```

### 3. Seed Initial Data (Optional)

```bash
./bin/shipping-migrate -seed
```

---

## Docker Deployment

### Build Production Images

```bash
# Build all targets
docker build --target api -t your-registry/shipping-api:v1.0.0 .
docker build --target worker -t your-registry/shipping-worker:v1.0.0 .
docker build --target migrate -t your-registry/shipping-migrate:v1.0.0 .

# With build arguments for versioning
docker build --target api \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t your-registry/shipping-api:v1.0.0 .
```

### Push to Registry

```bash
docker push your-registry/shipping-api:v1.0.0
docker push your-registry/shipping-worker:v1.0.0
docker push your-registry/shipping-migrate:v1.0.0
```

### Production Docker Compose

Create `docker-compose.production.yml`:

```yaml
version: '3.8'

services:
  api:
    image: your-registry/shipping-api:v1.0.0
    restart: always
    ports:
      - "8080:8080"
    environment:
      - APP_ENV=production
    env_file:
      - .env.production
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health/live"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"

  worker:
    image: your-registry/shipping-worker:v1.0.0
    restart: always
    environment:
      - APP_ENV=production
    env_file:
      - .env.production
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

### Deploy with Docker Compose

```bash
# Deploy
docker compose -f docker-compose.production.yml up -d

# View logs
docker compose -f docker-compose.production.yml logs -f

# Scale API servers
docker compose -f docker-compose.production.yml up -d --scale api=3
```

---

## Kubernetes Deployment

### Namespace and Secrets

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: shipping
---
# k8s/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: shipping-secrets
  namespace: shipping
type: Opaque
stringData:
  DATABASE_URL: "postgres://user:pass@host:5432/shipping?sslmode=require"
  REDIS_URL: "redis://:password@host:6379/0"
  ENCRYPTION_KEY: "<your-encryption-key>"
  JWT_SECRET: "<your-jwt-secret>"
  WEBHOOK_SIGNING_SECRET: "<your-webhook-secret>"
```

### ConfigMap

```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shipping-config
  namespace: shipping
data:
  APP_ENV: "production"
  APP_DEBUG: "false"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  API_PORT: "8080"
  RATE_LIMIT_ENABLED: "true"
  RATE_LIMIT_REQUESTS_PER_SECOND: "100"
  METRICS_ENABLED: "true"
  METRICS_PORT: "9090"
```

### API Deployment

```yaml
# k8s/api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shipping-api
  namespace: shipping
spec:
  replicas: 3
  selector:
    matchLabels:
      app: shipping-api
  template:
    metadata:
      labels:
        app: shipping-api
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: api
        image: your-registry/shipping-api:v1.0.0
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        envFrom:
        - configMapRef:
            name: shipping-config
        - secretRef:
            name: shipping-secrets
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2000m"
            memory: "2Gi"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: shipping-api
              topologyKey: kubernetes.io/hostname
---
apiVersion: v1
kind: Service
metadata:
  name: shipping-api
  namespace: shipping
spec:
  selector:
    app: shipping-api
  ports:
  - port: 80
    targetPort: 8080
    name: http
  - port: 9090
    targetPort: 9090
    name: metrics
```

### Worker Deployment

```yaml
# k8s/worker-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shipping-worker
  namespace: shipping
spec:
  replicas: 2
  selector:
    matchLabels:
      app: shipping-worker
  template:
    metadata:
      labels:
        app: shipping-worker
    spec:
      containers:
      - name: worker
        image: your-registry/shipping-worker:v1.0.0
        envFrom:
        - configMapRef:
            name: shipping-config
        - secretRef:
            name: shipping-secrets
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2000m"
            memory: "2Gi"
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
```

### Ingress

```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shipping-api
  namespace: shipping
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/rate-limit-window: "1s"
spec:
  tls:
  - hosts:
    - api.your-domain.com
    secretName: shipping-api-tls
  rules:
  - host: api.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: shipping-api
            port:
              number: 80
```

### Horizontal Pod Autoscaler

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: shipping-api-hpa
  namespace: shipping
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: shipping-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f k8s/

# Check deployment status
kubectl -n shipping get pods
kubectl -n shipping get services
kubectl -n shipping get ingress

# View logs
kubectl -n shipping logs -f deployment/shipping-api
kubectl -n shipping logs -f deployment/shipping-worker
```

---

## CI/CD Pipeline

### GitHub Actions Example

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    tags:
      - 'v*'

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.22'
    - name: Run tests
      run: make test

  build:
    needs: test
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
    - uses: actions/checkout@v4
    
    - name: Log in to Container Registry
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    
    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    
    - name: Build and push API image
      uses: docker/build-push-action@v5
      with:
        context: .
        target: api
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}-api:${{ github.ref_name }}
        build-args: |
          VERSION=${{ github.ref_name }}
          COMMIT=${{ github.sha }}
          BUILD_TIME=${{ github.event.head_commit.timestamp }}
    
    - name: Build and push Worker image
      uses: docker/build-push-action@v5
      with:
        context: .
        target: worker
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}-worker:${{ github.ref_name }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Deploy to Kubernetes
      uses: azure/k8s-deploy@v4
      with:
        namespace: shipping
        manifests: |
          k8s/api-deployment.yaml
          k8s/worker-deployment.yaml
        images: |
          ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}-api:${{ github.ref_name }}
          ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}-worker:${{ github.ref_name }}
```

---

## Monitoring & Observability

### Prometheus Metrics

The application exposes metrics at `/metrics` on port 9090:

- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency histogram
- `shipments_created_total` - Total shipments created
- `provider_api_calls_total` - Provider API call counts
- `provider_api_latency_seconds` - Provider API latency

### Grafana Dashboards

Import the provided dashboards from `configs/grafana/provisioning/dashboards/`.

### Alerting Rules

```yaml
# prometheus-alerts.yaml
groups:
- name: shipping-alerts
  rules:
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: High error rate detected
      
  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: High latency detected (p95 > 2s)
      
  - alert: WorkerQueueBacklog
    expr: asynq_queue_size{queue="default"} > 1000
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: Worker queue backlog growing
```

### Logging

Logs are output in JSON format for easy parsing:

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00Z",
  "caller": "handler/shipment.go:45",
  "msg": "shipment created",
  "trace_id": "abc123",
  "org_id": "org_xxx",
  "shipment_id": "shp_xxx"
}
```

Use a log aggregator like:

- ELK Stack (Elasticsearch, Logstash, Kibana)
- Loki + Grafana
- Datadog
- AWS CloudWatch

---

## Security Checklist

### Pre-Deployment

- [ ] Generate unique `ENCRYPTION_KEY` (32-byte hex)
- [ ] Generate unique `JWT_SECRET`
- [ ] Generate unique `WEBHOOK_SIGNING_SECRET`
- [ ] Set `APP_DEBUG=false`
- [ ] Set `APP_ENV=production`
- [ ] Enable SSL/TLS for database (`DB_SSLMODE=require`)
- [ ] Enable SSL/TLS for Redis
- [ ] Configure CORS with specific origins (not `*`)
- [ ] Review and restrict API rate limits

### Infrastructure

- [ ] Use HTTPS only (TLS 1.2+)
- [ ] Configure firewall rules
- [ ] Use private subnets for databases
- [ ] Enable database encryption at rest
- [ ] Enable S3 bucket encryption
- [ ] Use IAM roles instead of access keys where possible
- [ ] Enable audit logging

### Application

- [ ] All secrets stored in secret manager (not env files)
- [ ] API keys are hashed in database
- [ ] Provider credentials are encrypted with AES-256-GCM
- [ ] Webhook signatures are verified
- [ ] Input validation on all endpoints
- [ ] SQL injection prevention (using sqlc)

---

## Scaling Considerations

### Horizontal Scaling

**API Servers:**

- Stateless design allows unlimited horizontal scaling
- Use load balancer with health checks
- Consider geographic distribution for latency

**Workers:**

- Scale based on queue depth
- Monitor `asynq_queue_size` metric
- Consider dedicated workers for critical queues

### Database Scaling

**Read Replicas:**

- Route read queries to replicas
- Use connection pooling (PgBouncer)

**Partitioning:**

- Consider partitioning large tables by date:
  - `tracking_events`
  - `audit_logs`
  - `provider_api_logs`

### Caching Strategy

- Redis for session data
- Redis for rate limiting
- Redis for idempotency keys
- Consider CDN for static assets (labels)

---

## Backup & Recovery

### Database Backups

```bash
# Daily automated backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME -F c -f backup_$(date +%Y%m%d).dump

# Restore from backup
pg_restore -h $DB_HOST -U $DB_USER -d $DB_NAME backup_20240115.dump
```

### Point-in-Time Recovery

Enable WAL archiving for PostgreSQL:

```sql
-- postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://your-backup-bucket/wal/%f'
```

### Disaster Recovery

1. **RTO (Recovery Time Objective):** Target < 1 hour
2. **RPO (Recovery Point Objective):** Target < 5 minutes

**Strategy:**

- Continuous WAL archiving to S3
- Daily full backups
- Cross-region replication for S3 buckets
- Documented runbooks for recovery procedures

---

## Troubleshooting

### Common Issues

**Database Connection Errors:**

```bash
# Check connectivity
psql $DATABASE_URL -c "SELECT 1"

# Check connection pool
kubectl -n shipping exec -it deployment/shipping-api -- wget -qO- localhost:9090/metrics | grep db_pool
```

**Redis Connection Errors:**

```bash
# Check connectivity
redis-cli -u $REDIS_URL PING
```

**High Memory Usage:**

```bash
# Check Go memory stats
kubectl -n shipping exec -it deployment/shipping-api -- wget -qO- localhost:9090/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Health Endpoints

- `GET /health/live` - Liveness probe (is the process running?)
- `GET /health/ready` - Readiness probe (can it accept traffic?)

---

## Support

For issues and questions:

- Check application logs
- Review metrics dashboards
- Consult the API documentation at `/docs`
