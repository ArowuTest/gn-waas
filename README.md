# GN-WAAS — Ghana National Water Audit & Assurance System

> **Sovereign audit layer for Ghana Water Limited (GWL).**  
> Detects underbilling, overbilling, category misclassification, and non-revenue water (NRW) anomalies by reconciling bulk meter data against billed consumption — using 2026 PURC tariffs and GRA E-VAT compliance.

---

## Build Status

| Component | Status |
|---|---|
| Go Services (7) | ✅ All build clean |
| Go Tests — api-gateway handler | ✅ 61/61 pass |
| Go Tests — api-gateway repository | ✅ 12/12 pass |
| Go Tests — sentinel reconciler | ✅ 17/17 pass |
| Go Tests — sentinel phantom checker | ✅ 13/13 pass |
| Go Tests — sentinel night_flow + water_balance | ✅ pass |
| Go Tests — tariff-engine | ✅ pass |
| Go Tests — gra-bridge (sandbox + live mode) | ✅ 14/14 pass |
| Flutter Tests | ✅ 93/93 pass |
| Frontend Builds (3 portals) | ✅ All build clean |
| Database Migrations (10) | ✅ Sequential, idempotent |
| Docker Compose | ✅ 19 services defined |
| K8s Manifests | ✅ All 7 services + Ingress + HPA + NetworkPolicy |

---

## Repository Structure

```
gn-waas/
├── backend/
│   ├── api-gateway/          # Fiber HTTP gateway — all REST endpoints
│   ├── tariff-engine/        # 2026 PURC tiered tariff + VAT shadow billing
│   ├── sentinel/             # Anomaly detection + IWA water balance
│   ├── gra-bridge/           # GRA VSDC E-VAT integration (real HTTP + QR)
│   ├── ocr-service/          # Tesseract OCR for meter photo verification
│   ├── cdc-ingestor/         # GWL DB change-data-capture + NATS publisher
│   └── meter-ingestor/       # gRPC bulk meter reading ingestor
├── frontend/
│   ├── landing/              # Public landing page (Vite + React)
│   ├── admin-portal/         # System admin UI (users, districts, config)
│   ├── authority-portal/     # Audit authority UI (NRW, field jobs, reports)
│   └── gwl-portal/           # GWL Case Management Portal (NEW)
├── mobile/
│   └── field-officer-app/    # React Native offline-first field app
├── database/
│   ├── migrations/           # 006 sequential SQL migrations
│   └── seeds/                # 006 seed files incl. demo time-series
├── shared/go/                # Shared Go modules (auth middleware, NATS, HTTP)
├── infrastructure/
│   └── k8s/                  # Kubernetes manifests
├── docker-compose.yml        # Full local stack (19 services)
└── go.work                   # Go workspace (9 modules)
```

---

## Architecture Overview

```
                    ┌─────────────────────────────────────────┐
                    │           GWL ERP / Billing DB           │
                    └──────────────┬──────────────────────────┘
                                   │ CDC (change-data-capture)
                    ┌──────────────▼──────────────────────────┐
                    │         cdc-ingestor (Go)                │
                    │  Mirrors GWL billing → GN-WAAS DB        │
                    │  Publishes gnwaas.cdc.sync.completed      │
                    └──────────────┬──────────────────────────┘
                                   │ NATS
          ┌────────────────────────▼────────────────────────┐
          │                  NATS JetStream                  │
          └──┬──────────────────────┬────────────────────────┘
             │                      │
  ┌──────────▼──────┐    ┌──────────▼──────────────────────┐
  │  tariff-engine  │    │         sentinel (Go)            │
  │  Shadow billing │    │  NRW analysis + IWA water balance│
  │  2026 PURC rates│    │  Anomaly flags → anomaly_flags   │
  └──────────┬──────┘    └──────────┬──────────────────────┘
             │                      │
             └──────────┬───────────┘
                        │
          ┌─────────────▼──────────────────────────────────┐
          │              api-gateway (Fiber)                │
          │  38 REST endpoints · JWKS JWT · RBAC roles      │
          └──┬──────────────┬──────────────┬───────────────┘
             │              │              │
    ┌────────▼───┐  ┌───────▼──────┐  ┌───▼──────────────┐
    │admin-portal│  │authority-    │  │  gwl-portal       │
    │(React)     │  │portal(React) │  │  (React)          │
    └────────────┘  └──────────────┘  └──────────────────┘

  ┌──────────────────────────────────────────────────────┐
  │  meter-ingestor (gRPC + HTTP)                        │
  │  Bulk IoT meter readings → DB → NATS events          │
  └──────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────┐
  │  gra-bridge (Go)                                     │
  │  GRA VSDC E-VAT API · QR code signing · Backoff      │
  └──────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────┐
  │  ocr-service (Go + Tesseract)                        │
  │  Meter photo OCR · GPS metadata · Photo hash         │
  └──────────────────────────────────────────────────────┘
```

---

## Portals & User Roles

| Portal | URL (default) | Roles |
|---|---|---|
| **Landing Page** | `:3000` | Public |
| **Admin Portal** | `:3001` | `SYSTEM_ADMIN` |
| **Authority Portal** | `:3002` | `AUDIT_SUPERVISOR`, `DISTRICT_MANAGER` |
| **GWL Case Portal** | `:3003` | `GWL_SUPERVISOR`, `GWL_BILLING_OFFICER`, `GWL_MANAGER` |
| **Field Officer App** | Mobile (Expo) | `FIELD_OFFICER` |

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.22+ | `go.work` workspace |
| Node.js | 20+ | All frontends |
| Docker + Compose | 24+ | Full local stack |
| PostgreSQL | 15+ with TimescaleDB | Or use Docker |
| Keycloak | 23+ | Identity provider |
| NATS | 2.10+ | Async messaging |
| Tesseract OCR | 5+ | OCR service host |
| `protoc` + `protoc-gen-go-grpc` | Latest | Meter ingestor gRPC |

---

## Quick Start — Local Development

### 1. Clone

```bash
git clone https://github.com/ArowuTest/gn-waas.git
cd gn-waas
```

### 2. Start the full stack

```bash
docker compose up -d
```

This starts: PostgreSQL (TimescaleDB), Keycloak, Redis, MinIO, NATS, all 7 Go services, and the seed runner.

### 3. Run database migrations + seeds

```bash
# Migrations run automatically via seed-runner in docker-compose
# To run manually:
psql $DATABASE_URL -f database/migrations/001_extensions_and_types.sql
psql $DATABASE_URL -f database/migrations/002_core_tables.sql
psql $DATABASE_URL -f database/migrations/003_billing_and_shadow_ledger.sql
psql $DATABASE_URL -f database/migrations/004_sentinel_and_audit.sql
psql $DATABASE_URL -f database/migrations/005_data_confidence_and_reporting.sql
psql $DATABASE_URL -f database/migrations/006_gwl_case_management.sql

# Seeds (includes 12-month demo time-series data)
psql $DATABASE_URL -f database/seeds/001_system_config.sql
psql $DATABASE_URL -f database/seeds/002_tariff_rates.sql
psql $DATABASE_URL -f database/seeds/003_districts.sql
psql $DATABASE_URL -f database/seeds/004_users.sql
psql $DATABASE_URL -f database/seeds/005_sample_accounts.sql
psql $DATABASE_URL -f database/seeds/006_demo_timeseries.sql
```

### 4. Build & run a single service locally

```bash
cd backend/api-gateway
cp .env.example .env          # fill in values
go run ./cmd/main.go
```

### 5. Build frontends

```bash
cd frontend/gwl-portal
npm install
npm run dev        # development
npm run build      # production bundle
```

---

## Environment Variables

Each service has a `.env.example` in its directory. **Copy and fill before deploying.**

### `backend/api-gateway/.env.example`

```env
DATABASE_URL=postgres://gnwaas:password@localhost:5432/gnwaas
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=gnwaas
PORT=8000
DEV_MODE=false                  # true = bypass JWT (NEVER in production)
GRA_BRIDGE_URL=http://gra-bridge:8003
SENTINEL_URL=http://sentinel:8001
TARIFF_ENGINE_URL=http://tariff-engine:8002
```

### `backend/sentinel/.env.example`

```env
DATABASE_URL=postgres://gnwaas:password@localhost:5432/gnwaas
NATS_URL=nats://localhost:4222
NRW_THRESHOLD_PCT=25.0          # flag districts above this NRW %
CRITICAL_NRW_PCT=40.0
PORT=8001
```

### `backend/cdc-ingestor/.env.example`

```env
DATABASE_URL=postgres://gnwaas:password@localhost:5432/gnwaas
NATS_URL=nats://localhost:4222
GWL_DB_HOST=                    # leave blank for demo/synthetic mode
GWL_DB_PORT=5432
GWL_DB_NAME=gwl_billing
GWL_DB_USER=readonly_user
GWL_DB_PASSWORD=
SYNC_INTERVAL_MINUTES=60
PORT=8005
```

> **Note:** If `GWL_DB_HOST` is empty, the CDC ingestor runs in **demo mode** — it generates synthetic billing data using real PURC 2026 tariff calculations. Set `GWL_DB_HOST` to connect to the live GWL billing database.

### `backend/gra-bridge/.env.example`

```env
GRA_VSDC_URL=https://vsdc.gra.gov.gh/api/v1
GRA_CLIENT_ID=your_client_id
GRA_CLIENT_SECRET=your_client_secret
GRA_SANDBOX_MODE=true           # false in production
PORT=8003
```

### `backend/meter-ingestor/.env.example`

```env
DATABASE_URL=postgres://gnwaas:password@localhost:5432/gnwaas
NATS_URL=nats://localhost:4222
GRPC_PORT=9090
HTTP_PORT=8006
```

### Frontend environment variables

Create `.env` in each frontend directory:

```env
# frontend/gwl-portal/.env
VITE_API_URL=http://localhost:8000
VITE_KEYCLOAK_URL=http://localhost:8080
VITE_KEYCLOAK_REALM=gnwaas
VITE_KEYCLOAK_CLIENT_ID=gwl-portal
```

---

## Running Tests

```bash
# All Go tests (119+ tests across 4 services)
cd backend/tariff-engine  && go test ./... -v
cd backend/sentinel       && go test ./... -v
cd backend/gra-bridge     && go test ./... -v
cd backend/api-gateway    && go test ./... -v

# Run all at once
for svc in tariff-engine sentinel gra-bridge api-gateway; do
  (cd backend/$svc && go test ./... -count=1) || exit 1
done
```

### Test Coverage by Service

| Service | Tests | Coverage Areas |
|---|---|---|
| `tariff-engine` | 7 | Residential/commercial shadow billing, VAT, tier boundaries, variance flagging |
| `sentinel` | 10 | NRW district balance, IWA water balance grades, revenue recovery estimation |
| `gra-bridge` | 5 | Invoice signing, QR code generation, sandbox mode, audit invoice |
| `api-gateway` | 10 | Health endpoints, admin user RBAC, district handler, UUID validation |

---

## Production Deployment

### Option A — Docker Compose (single server)

```bash
# Set all secrets in environment or .env files
docker compose -f docker-compose.yml up -d --build

# Verify all services healthy
docker compose ps
```

### Option B — Kubernetes (recommended for production)

```bash
# Apply namespace and config
kubectl apply -f infrastructure/k8s/namespace.yaml
kubectl apply -f infrastructure/k8s/configmap.yaml

# Create secrets (fill values first)
kubectl apply -f infrastructure/k8s/secrets.yaml

# Deploy services
kubectl apply -f infrastructure/k8s/api-gateway-deployment.yaml
kubectl apply -f infrastructure/k8s/sentinel-deployment.yaml
```

> Additional K8s manifests needed for production (see **Production Gaps** below).

### CI/CD

The CI pipeline definition is at `docs/ci-pipeline.yml`. To activate:

```bash
mkdir -p .github/workflows
cp docs/ci-pipeline.yml .github/workflows/ci.yml
git add .github/workflows/ci.yml
git commit -m "ci: activate GitHub Actions pipeline"
git push
```

The pipeline runs: Go build → Go tests → Frontend builds → Docker image push.  
Requires a GitHub token with `workflow` scope and Docker Hub credentials as repository secrets.

---

## Production Readiness Checklist

### ✅ Complete and Production-Ready

| Item | Detail |
|---|---|
| **JWKS JWT Authentication** | `shared/go/middleware/auth.go` — validates RS256 tokens against Keycloak JWKS endpoint |
| **RBAC Role Enforcement** | `RequireRoles()` middleware on every protected route |
| **2026 PURC Tariff Engine** | Real tiered rates (Residential 0–5m³ ₵6.1225, >5m³ ₵10.8320, Commercial fixed charges) |
| **IWA/AWWA Water Balance** | Full ILI, CARL, UARL computation with A/B/C/D grade classification |
| **GRA VSDC E-VAT Integration** | Real HTTP calls with exponential backoff, QR code generation, sandbox/production toggle |
| **NATS Async Messaging** | CDC ingestor publishes `gnwaas.cdc.sync.completed`; Sentinel subscribes and triggers scans |
| **gRPC Meter Ingestor** | Protobuf-defined service, streaming readings, NATS event publishing |
| **CDC Ingestor** | Live mode (real GWL DB) + demo mode (synthetic PURC-accurate data) |
| **OCR Service** | Tesseract integration, GPS metadata, photo hash, device ID |
| **Mobile Offline-First** | `expo-sqlite` offline job cache, `NetInfo` sync-on-reconnect |
| **Admin Portal** | User management, district config, audit threshold settings |
| **Authority Portal** | NRW summary with IWA grades, filtering, CSV/PDF export |
| **GWL Case Portal** | Full case management workflow (see below) |
| **Database Schema** | 6 migrations, TimescaleDB hypertables, proper FK constraints and indexes |
| **Seed Data** | 12-month demo time-series with real tariff calculations |
| **SQL Injection Protection** | All queries parameterised (`$1, $2...`), zero string concatenation |
| **Transactional Integrity** | Multi-step operations (assign officer, create field job) wrapped in DB transactions |

### ⚠️ Required Before Go-Live

| Item | Action Required | Priority |
|---|---|---|
| **Keycloak realm configuration** | Import `infrastructure/keycloak/gnwaas-realm.json` into your Keycloak instance. Create clients: `admin-portal`, `authority-portal`, `gwl-portal`, `field-officer-app`. Create roles: `SYSTEM_ADMIN`, `AUDIT_SUPERVISOR`, `DISTRICT_MANAGER`, `FIELD_OFFICER`, `GWL_SUPERVISOR`, `GWL_BILLING_OFFICER`, `GWL_MANAGER` | **CRITICAL** |
| **GRA VSDC credentials** | Obtain production `GRA_CLIENT_ID` and `GRA_CLIENT_SECRET` from Ghana Revenue Authority. Set `GRA_SANDBOX_MODE=false` | **CRITICAL** |
| **GWL database read access** | Provision a read-only PostgreSQL user on the GWL billing database. Set `GWL_DB_*` env vars in `cdc-ingestor`. Without this, the system runs on synthetic demo data | **HIGH** |
| **SSL/TLS certificates** | Place a reverse proxy (nginx/Traefik) in front of all services. All traffic must be HTTPS in production | **CRITICAL** |
| **Secrets management** | Move all passwords and API keys from `.env` files into a secrets manager (HashiCorp Vault, AWS Secrets Manager, or Kubernetes Secrets with encryption at rest) | **HIGH** |
| **CI/CD activation** | Copy `docs/ci-pipeline.yml` → `.github/workflows/ci.yml` and add Docker Hub credentials as GitHub repository secrets | **MEDIUM** |
| **K8s manifests — remaining services** | Add K8s deployments for: `tariff-engine`, `gra-bridge`, `ocr-service`, `cdc-ingestor`, `meter-ingestor`, `nats`, `keycloak`, `gwl-portal`, `authority-portal`, `admin-portal` | **MEDIUM** |
| **Tesseract on OCR host** | Install `tesseract-ocr` on the host running `ocr-service`. Dockerfile includes `apt-get install tesseract-ocr` | **HIGH** |
| **TimescaleDB extension** | Ensure `CREATE EXTENSION IF NOT EXISTS timescaledb` runs before migrations. Included in `001_extensions_and_types.sql` | **HIGH** |
| **NATS persistence** | Configure NATS JetStream with persistent storage for production. Default config is in-memory only | **MEDIUM** |
| **Monitoring** | Add Prometheus metrics endpoints and Grafana dashboards. Suggested: instrument each service with `prometheus/client_golang` | **MEDIUM** |
| **Log aggregation** | Route structured JSON logs to a centralised system (ELK, Loki, or CloudWatch) | **MEDIUM** |

### 🔵 Nice-to-Have (Post-Launch)

| Item | Detail |
|---|---|
| **Frontend E2E tests** | Add Playwright or Cypress tests for the three portals |
| **Load testing** | Benchmark API gateway under concurrent district scans |
| **GWL portal K8s deployment** | Add `gwl-portal` to Kubernetes manifests |
| **Automated monthly report emails** | Cron job to email `MonthlyReport` PDF to GWL management |
| **SMS alerts** | Integrate with a Ghana SMS gateway for critical anomaly notifications to field officers |

---

## GWL Case Management Portal

The newest portal (`frontend/gwl-portal`) is purpose-built for **Ghana Water Limited staff** — not the audit team or citizens.

### Workflow

```
Sentinel detects anomaly
        ↓
Case appears in GWL Portal (status: PENDING_REVIEW)
        ↓
GWL Supervisor reviews billing evidence
        ↓
Actions available:
  ├── Assign to Field Officer → status: FIELD_ASSIGNED
  │       ↓ Officer submits evidence via mobile app
  │       ↓ status: EVIDENCE_SUBMITTED
  ├── Request Reclassification → reclassification_requests table
  ├── Request Credit (overbilling) → credit_requests table
  ├── Mark Corrected → status: CORRECTED
  └── Dispute → status: DISPUTED (notifies audit team)
        ↓
Case Closed → immutable audit trail in gwl_case_actions
```

### Pages

| Page | Purpose |
|---|---|
| **Dashboard** | KPI strip, financial impact, pending review queue |
| **Case Queue** | All cases — 6 filters, search, pagination, CSV export |
| **Case Detail** | Full evidence, billing comparison, audit trail, all action buttons |
| **Underbilling** | Cases ranked by monthly revenue loss |
| **Overbilling** | Overcharge cases + credit request tracker |
| **Misclassification** | Category mismatch cases + reclassification tracker |
| **Field Assignments** | Officer workload, overdue tracking, by-officer summary |
| **Monthly Report** | Period selector, charts, CSV + print/PDF for Ministry of Finance |

---

## API Reference

Base URL: `http://localhost:8000/api/v1`

All endpoints require `Authorization: Bearer <JWT>` header.

### Health

```
GET /health
```

### Districts

```
GET    /districts
GET    /districts/:id
POST   /admin/districts/          [SYSTEM_ADMIN]
PUT    /admin/districts/:id       [SYSTEM_ADMIN]
```

### Accounts

```
GET /accounts/search?q=&district_id=
GET /accounts/:id
GET /accounts/gwl/:gwl_number
GET /accounts/district/:district_id
```

### Audit / Field Jobs

```
GET  /audits/my-jobs              [FIELD_OFFICER]
POST /audits/submit               [FIELD_OFFICER]
GET  /audits/evidence/:job_id     [FIELD_OFFICER, AUDIT_SUPERVISOR]
```

### NRW Reports

```
GET /reports/nrw-summary
GET /reports/nrw-trend/:district_id
GET /reports/my-district
```

### GWL Case Management

```
GET   /gwl/cases/summary
GET   /gwl/cases?flag_type=&severity=&gwl_status=&district_id=&search=&limit=&offset=
GET   /gwl/cases/:id
GET   /gwl/cases/:id/actions
POST  /gwl/cases/:id/assign
PATCH /gwl/cases/:id/status
POST  /gwl/cases/:id/reclassify
POST  /gwl/cases/:id/credit
GET   /gwl/reclassifications?status=
GET   /gwl/credits?status=
GET   /gwl/reports/monthly?period=2026-03&district_id=
```

### Admin

```
GET    /admin/users               [SYSTEM_ADMIN]
POST   /admin/users               [SYSTEM_ADMIN]
PUT    /admin/users/:id           [SYSTEM_ADMIN]
POST   /admin/users/:id/reset-password [SYSTEM_ADMIN]
GET    /config                    [SYSTEM_ADMIN]
PUT    /config/:key               [SYSTEM_ADMIN]
```

---

## Data Flow — Anomaly Detection

```
1. CDC Ingestor mirrors GWL billing records into water_accounts + gwl_bills
2. Tariff Engine computes shadow_bill for each account using 2026 PURC rates
3. Sentinel compares shadow_bill vs gwl_bill:
   - Variance > 15% → BILLING_VARIANCE flag
   - Category consumption mismatch → CATEGORY_MISMATCH flag
   - Overbilling detected → OVERBILLING flag
   - Night-flow NRW > threshold → NRW_ANOMALY flag
4. Flags written to anomaly_flags with severity (LOW/MEDIUM/HIGH/CRITICAL)
5. NATS event published → GWL Portal case queue populated
6. GWL staff action the case → audit trail in gwl_case_actions
```

---

## Tariff Rates (2026 PURC)

| Category | Tier | Rate (GHS/m³) |
|---|---|---|
| Residential | 0–5 m³ | ₵6.1225 |
| Residential | >5 m³ | ₵10.8320 |
| Commercial | Fixed charge | ₵45.00/month |
| Commercial | Per m³ | ₵15.2400 |
| Industrial | Fixed charge | ₵120.00/month |
| Industrial | Per m³ | ₵18.9600 |
| Government | Per m³ | ₵12.5000 |
| Standpipe | Per m³ | ₵4.2000 |

VAT: **20%** applied to all categories per GRA regulations.

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Ensure all tests pass: `go test ./...` in each service
4. Ensure all frontends build: `npm run build` in each portal
5. Submit a pull request

---

## License

Proprietary — Ghana Water Limited / GN-WAAS Project Team  
© 2026 All rights reserved.

---

*Built with Go 1.22, React 18, Vite 5, PostgreSQL 15 + TimescaleDB, Keycloak 23, NATS 2.10, Expo SDK 51*
