# GN-WAAS — Ghana National Water Audit & Assurance System

> Reducing Ghana's 51.6% Non-Revenue Water through intelligent billing audit, fraud detection, and GRA-compliant revenue recovery.

[![CI/CD](https://github.com/ArowuTest/gn-waas/actions/workflows/ci.yml/badge.svg)](https://github.com/ArowuTest/gn-waas/actions)

---

## Problem Statement

Ghana Water Limited (GWL) loses **51.6% of all water produced** as Non-Revenue Water (NRW).
The commercial loss component — phantom meters, ghost accounts, category fraud, under-billing —
represents recoverable revenue estimated at **GHS 120M+ annually**.

GN-WAAS is a sovereign audit layer that:
1. **Mirrors** GWL's billing database via CDC (read-only replica)
2. **Recalculates** every bill using PURC 2026 tariffs (shadow billing)
3. **Flags** variances, phantom meters, and fraud patterns (Sentinel)
4. **Dispatches** field officers for blind audits with GPS-locked evidence
5. **Submits** confirmed losses to GRA VSDC API for VAT recovery
6. **Reports** NRW using the IWA/AWWA Water Balance framework

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Admin Portal (React 19)                   │
│              Dashboard · Anomalies · Audits · NRW           │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS
┌──────────────────────────▼──────────────────────────────────┐
│                   API Gateway (Go/Fiber)                     │
│              Auth (Keycloak JWT) · RBAC · Routing           │
└──┬──────────┬──────────┬──────────┬──────────┬─────────────┘
   │          │          │          │          │
   ▼          ▼          ▼          ▼          ▼
Tariff    Sentinel    GRA        OCR       CDC
Engine    Service     Bridge     Service   Ingestor
(3001)    (3002)      (3003)     (3005)    (3006)
   │          │          │
   └──────────┴──────────┘
              │
   ┌──────────▼──────────┐
   │  PostgreSQL 16 +    │
   │  TimescaleDB        │
   │  (gnwaas DB)        │
   └─────────────────────┘
```

---

## Repository Structure

```
gn-waas/
├── apps/
│   └── admin-portal/          # React 19 + TypeScript + Tailwind
├── services/
│   ├── api-gateway/           # Central HTTP gateway (Go/Fiber)
│   ├── tariff-engine/         # PURC 2026 shadow billing (Go)
│   ├── sentinel/              # Fraud detection engine (Go)
│   ├── gra-bridge/            # GRA VSDC API integration (Go)
│   ├── ocr-service/           # Tesseract meter OCR (Go)
│   └── cdc-ingestor/          # GWL database CDC (Go)
├── pkg/
│   └── shared/                # Shared Go packages
│       ├── database/          # pgxpool + error handling
│       ├── http/response/     # Standard API envelope
│       ├── middleware/        # Auth + RBAC + logging
│       └── pagination/        # Generic pagination
├── database/
│   ├── migrations/            # 5 ordered SQL migration files
│   └── seeds/                 # 5 seed data files
├── infrastructure/
│   ├── keycloak/              # Realm export JSON
│   ├── k8s/                   # Kubernetes manifests
│   └── ci/                    # CI/CD helpers
├── docs/                      # Per-phase documentation
├── config/
│   └── gwl_schema_map.yaml    # GWL → GN-WAAS column mapping
├── scripts/
│   └── init-db.sh             # Database initialisation script
└── docker-compose.yml         # Local development environment
```

---

## Quick Start (Local Development)

### Prerequisites
- Docker + Docker Compose
- Go 1.22+
- Node 22+

### 1. Start Infrastructure
```bash
docker-compose up -d postgres redis keycloak minio
```

### 2. Initialise Database
```bash
./scripts/init-db.sh
```

### 3. Start Backend Services
```bash
# Terminal 1: Tariff Engine
cd services/tariff-engine && go run ./cmd/main.go

# Terminal 2: Sentinel
cd services/sentinel && go run ./cmd/main.go

# Terminal 3: API Gateway
cd services/api-gateway && go run ./cmd/main.go
```

### 4. Start Admin Portal
```bash
cd apps/admin-portal
npm install && npm run dev
# Open http://localhost:5173
```

### 5. Login (Dev Mode)
Use the quick-access buttons on the login page to sign in as any role.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_NAME` | `gnwaas` | Database name |
| `DB_USER` | `gnwaas_user` | Database user |
| `DB_PASSWORD` | — | Database password |
| `KEYCLOAK_URL` | `http://localhost:8080` | Keycloak URL |
| `KEYCLOAK_REALM` | `gnwaas` | Keycloak realm |
| `GRA_SANDBOX_MODE` | `true` | Use GRA sandbox API |
| `GRA_API_KEY` | — | GRA VSDC API key |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO endpoint |

---

## RBAC Roles

| Role | Description | Key Permissions |
|------|-------------|-----------------|
| `SYSTEM_ADMIN` | GN-WAAS operations team | Full access |
| `DISTRICT_MANAGER` | GWL district managers | District data + audit assignment |
| `AUDIT_SUPERVISOR` | Audit team leads | Audit management + field dispatch |
| `FIELD_OFFICER` | Field auditors | My jobs + evidence capture + SOS |
| `GRA_LIAISON` | GRA staff | GRA compliance + VAT reports |
| `FINANCE_ANALYST` | Finance team | NRW reports + recovery tracking |
| `READONLY_VIEWER` | GWL management | Read-only dashboard access |

---

## Business Model

| Revenue Stream | Rate | Trigger |
|----------------|------|---------|
| One-time deployment fee | GHS 2.5M | Contract signing |
| Monthly retainer | GHS 150K/month | Post-deployment |
| Success fee | 3% of recovered revenue | Per confirmed recovery |

---

## Compliance

- **GRA VSDC API**: All audit invoices signed with GRA QR code before locking
- **IWA/AWWA Water Balance**: NRW reporting aligned with international standards
- **AWWA Data Confidence Grade**: A–F grading on all district data
- **NITA Ghana**: Deployed on NITA-certified data centres
- **Keycloak**: Self-hosted identity provider (data sovereignty)

---

## Documentation

| Document | Description |
|----------|-------------|
| [Phase 1: Database](docs/PHASE_1_DATABASE.md) | Migrations, seed data, schema design |
| [Phase 2-7: Backend Services](docs/PHASE_2_BACKEND_SERVICES.md) | All Go services, RBAC, API reference |
| [Phase 8: Admin Portal](docs/PHASE_8_ADMIN_PORTAL.md) | React app, design system, pages |

---

## Built by
**ArowuTest** — Ghana Water Technology Solutions  
*Sovereign software. Ghana-owned. NITA-hosted.*
