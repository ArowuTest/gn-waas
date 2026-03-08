# GN-WAAS Pre-Deployment Checklist
**Version:** v26 → Production  
**Last Updated:** 2026-03-08  
**Owner:** DevOps / Infrastructure Team

---

## ⚠️ MANDATORY — System will not start without these

### 1. Create `.env` from `.env.example`

```bash
cp .env.example .env
```

Then fill in **every** value. The `:?` syntax in `docker-compose.yml` means Docker will
**refuse to start** if any of these are missing or empty:

| Variable | Requirement | Example format |
|---|---|---|
| `POSTGRES_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `GNWAAS_APP_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `GNWAAS_ADMIN_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `REDIS_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `MINIO_ROOT_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `KEYCLOAK_ADMIN_PASSWORD` | Min 20 chars, random | `openssl rand -base64 24` |
| `GRAFANA_ADMIN_PASSWORD` | Min 16 chars, random | `openssl rand -base64 18` |
| `APP_ENV` | Must be `production` | `production` |
| `DEV_MODE` | Must be `false` | `false` |
| `GRA_SANDBOX_MODE` | `false` for live GRA | `false` |

**Generate all secrets at once:**
```bash
echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "GNWAAS_APP_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "GNWAAS_ADMIN_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "REDIS_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "MINIO_ROOT_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 24)" >> .env
echo "GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 18)" >> .env
echo "APP_ENV=production" >> .env
echo "DEV_MODE=false" >> .env
echo "GRA_SANDBOX_MODE=false" >> .env
```

---

### 2. Keycloak Realm Setup (SEC-C01 — CRITICAL)

The Keycloak realm must be configured with **exactly** these role names.
Any mismatch causes silent downgrade to `ANONYMOUS`.

**Step 1:** Import the realm export:
```bash
# After Keycloak starts:
docker exec gnwaas-keycloak /opt/keycloak/bin/kc.sh import \
  --file /opt/keycloak/data/import/realm-export.json
```

**Step 2:** Verify roles exist in Keycloak Admin Console:
- URL: `https://auth.gnwaas.nita.gov.gh/admin`
- Realm: `gnwaas` → Realm roles

Required roles (must match exactly, case-sensitive):
```
SUPER_ADMIN       SYSTEM_ADMIN      MINISTER_VIEW
GRA_OFFICER       MOF_AUDITOR       GWL_EXECUTIVE
GWL_MANAGER       GWL_SUPERVISOR    GWL_ANALYST
FIELD_SUPERVISOR  FIELD_OFFICER     MDA_USER
```

**Step 3:** Configure User Attribute Mappers for each user:
- `gnwaas_role` → User Attribute → Token Claim Name: `gnwaas_role`
- `district_id` → User Attribute → Token Claim Name: `district_id`

**Step 4:** Assign roles to users via Keycloak Admin Console.
Each user must have:
- Exactly **one** realm role assigned
- `gnwaas_role` user attribute set to the same role string
- `district_id` user attribute set to their district UUID (leave blank for SUPER_ADMIN/SYSTEM_ADMIN)

---

### 3. Run Database Migrations

```bash
# Run all 33 migrations in order
docker compose run --rm migrate

# Verify migration 033 applied (RLS on meter_readings, gwl_billing_records, gra_compliance_log)
docker compose exec postgres psql -U gnwaas_admin -d gnwaas -c \
  "SELECT tablename, policyname FROM pg_policies WHERE tablename IN ('meter_readings','gwl_billing_records','gra_compliance_log');"
```

Expected output: 3 rows, one policy per table.

---

### 4. GRA VSDC API Credentials

```bash
# Add to .env:
GRA_VSDC_API_KEY=<your-live-GRA-VSDC-key>
GRA_VSDC_BASE_URL=https://vsdc.gra.gov.gh/api/v1
GRA_SANDBOX_MODE=false
```

**Test the connection before go-live:**
```bash
docker compose run --rm gra-bridge /app/gra-bridge --test-connection
```

---

### 5. Build Mobile APK for Field Officers

```bash
cd mobile/field-officer-app-flutter

# Production build with real API URL
flutter build apk --release \
  --dart-define=API_BASE_URL=https://api.gnwaas.nita.gov.gh/api/v1 \
  --dart-define=KEYCLOAK_URL=https://auth.gnwaas.nita.gov.gh \
  --dart-define=KEYCLOAK_REALM=gnwaas

# Output: build/app/outputs/flutter-apk/app-release.apk
```

**Distribute via:**
- Internal MDM (recommended for sovereign deployment)
- Direct APK sideload to field officer devices

---

## ✅ RECOMMENDED — Do before go-live

### 6. Smoke Test on Real Android Devices

Test on at least 2 physical Android devices (Android 10+):

- [ ] Login with a `FIELD_OFFICER` Keycloak account
- [ ] GPS geofence triggers correctly within 100m of a test meter
- [ ] Camera opens and OCR reads a meter number
- [ ] Photo uploads to MinIO successfully
- [ ] Job submission reaches the backend
- [ ] Outcome recording screen appears after submission
- [ ] Offline mode: disable WiFi, submit a job, re-enable, verify sync

### 7. Load Test the Sentinel Service

```bash
# Install k6
docker run --rm -i grafana/k6 run - < infrastructure/load-tests/sentinel_load.js
```

Target: 500 concurrent meter readings/minute without >200ms p95 latency.

### 8. Verify GRA VSDC Sandbox End-to-End

Before switching `GRA_SANDBOX_MODE=false`:
1. Create a test audit event
2. Confirm it via the admin portal
3. Verify a QR-code signed PDF is generated
4. Verify the GRA VSDC API returns a valid receipt

### 9. SSL/TLS Certificates

Ensure valid certificates are in place for:
- `api.gnwaas.nita.gov.gh`
- `auth.gnwaas.nita.gov.gh`
- `admin.gnwaas.nita.gov.gh`

```bash
# Check cert expiry
echo | openssl s_client -connect api.gnwaas.nita.gov.gh:443 2>/dev/null | \
  openssl x509 -noout -dates
```

### 10. Backup Configuration

```bash
# Verify automated backups are configured
docker compose exec postgres pg_dumpall -U gnwaas_admin | gzip > /backups/gnwaas_$(date +%Y%m%d).sql.gz

# Test restore on a separate instance before go-live
```

---

## 📋 Go/No-Go Sign-Off

| Check | Owner | Status |
|---|---|---|
| `.env` secrets set (all 10 variables) | DevOps | ☐ |
| Keycloak realm imported and roles verified | DevOps | ☐ |
| All 33 migrations applied successfully | DevOps | ☐ |
| RLS policies verified on 3 new tables | DevOps | ☐ |
| GRA VSDC live credentials configured | Project Manager | ☐ |
| Mobile APK built with production URLs | DevOps | ☐ |
| Smoke test on 2 physical Android devices | QA | ☐ |
| SSL certificates valid (>30 days) | DevOps | ☐ |
| Backup/restore tested | DevOps | ☐ |
| `DEV_MODE=false` confirmed in running container | DevOps | ☐ |

**All 10 checks must be ✅ before production traffic is directed to the system.**

---

## 🚨 Rollback Plan

If a critical issue is found after go-live:

```bash
# 1. Stop all services
docker compose down

# 2. Restore database from last backup
docker compose exec postgres psql -U gnwaas_admin -d gnwaas < /backups/gnwaas_YYYYMMDD.sql

# 3. Roll back to previous image tag
docker compose up -d --scale api-gateway=0
docker compose up -d api-gateway:v25

# 4. Notify GRA of temporary service interruption
```

---

*This checklist was generated from the v26 Production-Readiness Audit findings.*  
*SEC-C01, INFRA-H01, RLS-M01 have been remediated in code as of commit f9e4a0d8.*  
*DB-M01 (migration idempotency) was already fixed in migrations 012 and 025.*
