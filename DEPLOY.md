# GN-WAAS Deployment Guide — Render + Vercel

## Architecture

```
Vercel (3 frontends)          Render (backend)
─────────────────────         ──────────────────────────────────
gn-waas-admin.vercel.app  ──► gnwaas-api.onrender.com  (API Gateway)
gn-waas-gwl.vercel.app    ──►        │
gn-waas-authority.vercel.app ──►     ├── gnwaas-sentinel (pserv)
                                     ├── gnwaas-tariff-engine (pserv)
                                     ├── gnwaas-gra-bridge (pserv)
                                     ├── gnwaas-postgres (managed DB)
                                     └── gnwaas-redis (managed Redis)
```

---

## Step 1 — Deploy Backend on Render

### 1a. Connect your repo

1. Go to [render.com](https://render.com) → **New** → **Blueprint**
2. Connect your GitHub account and select `ArowuTest/gn-waas`
3. Render will detect `render.yaml` and show all services to create
4. Click **Apply**

### 1b. Set secret environment variables

After the blueprint creates the services, go to each service and set these in **Environment**:

| Service | Variable | Value |
|---------|----------|-------|
| `gnwaas-api` | `DB_PASSWORD` | A strong password (e.g. `gnwaas_app_prod_2026!`) |
| `gnwaas-api` | `REDIS_PASSWORD` | A strong password |
| `gnwaas-api` | `CORS_ALLOWED_ORIGINS` | *(set after Vercel deploy — see Step 3)* |
| `gnwaas-sentinel` | `DB_PASSWORD` | Same as above |
| `gnwaas-sentinel` | `REDIS_PASSWORD` | Same as above |
| `gnwaas-tariff-engine` | `DB_PASSWORD` | Same as above |
| `gnwaas-gra-bridge` | `DB_PASSWORD` | Same as above |
| `gnwaas-migrate` | `GNWAAS_ADMIN_PASSWORD` | Admin DB password |
| `gnwaas-migrate` | `GNWAAS_APP_PASSWORD` | Same as `DB_PASSWORD` above |

> **Tip:** Use Render's **Environment Groups** to share variables across services.

### 1c. Run database migrations

1. Go to **gnwaas-migrate** service in Render dashboard
2. Click **Trigger Run** (or wait for the scheduled run)
3. Watch the logs — you should see 28 migrations + seed data applied
4. ✅ When you see `Database setup complete!` the DB is ready

### 1d. Note your API Gateway URL

After `gnwaas-api` deploys, copy its URL — it will look like:
```
https://gnwaas-api.onrender.com
```

---

## Step 2 — Deploy Frontends on Vercel

Deploy each portal as a **separate Vercel project** pointing to the same repo.

### Admin Portal

1. Go to [vercel.com](https://vercel.com) → **New Project** → Import `ArowuTest/gn-waas`
2. **Root Directory**: `frontend/admin-portal`
3. **Framework**: Vite (auto-detected)
4. **Environment Variables**:
   ```
   VITE_API_URL=https://gnwaas-api.onrender.com
   VITE_DEV_MODE=false
   ```
5. Click **Deploy**

### GWL Portal

1. **New Project** → same repo → **Root Directory**: `frontend/gwl-portal`
2. **Environment Variables**:
   ```
   VITE_API_URL=https://gnwaas-api.onrender.com
   VITE_DEV_MODE=false
   ```

### Authority Portal

1. **New Project** → same repo → **Root Directory**: `frontend/authority-portal`
2. **Environment Variables**:
   ```
   VITE_API_URL=https://gnwaas-api.onrender.com
   VITE_DEV_MODE=false
   ```

---

## Step 3 — Wire CORS

After all 3 Vercel projects deploy, copy their URLs and update the API gateway:

1. Go to Render → `gnwaas-api` → **Environment**
2. Set `CORS_ALLOWED_ORIGINS`:
   ```
   https://gn-waas-admin.vercel.app,https://gn-waas-gwl.vercel.app,https://gn-waas-authority.vercel.app
   ```
   *(replace with your actual Vercel URLs)*
3. Click **Save Changes** — Render will redeploy automatically

---

## Step 4 — Verify

| Check | URL |
|-------|-----|
| API health | `https://gnwaas-api.onrender.com/health` |
| Admin portal | `https://gn-waas-admin.vercel.app` |
| GWL portal | `https://gn-waas-gwl.vercel.app` |
| Authority portal | `https://gn-waas-authority.vercel.app` |

Login with the seeded demo users:

| Role | Email | Password |
|------|-------|----------|
| System Admin | `admin@gnwaas.gov.gh` | `GnWaas2026!` |
| Audit Manager | `audit.manager@gnwaas.gov.gh` | `GnWaas2026!` |
| GWL Management | `gwl.manager@gwl.com.gh` | `GnWaas2026!` |
| Field Supervisor | `supervisor@gnwaas.gov.gh` | `GnWaas2026!` |
| Field Officer | `officer1@gnwaas.gov.gh` | `GnWaas2026!` |

> Or use the **"Development Quick Login"** buttons (DEV_MODE=true is set by default).

---

## Free Tier Limitations

| Service | Render Free Tier Limit |
|---------|----------------------|
| Web services | Spin down after 15 min inactivity (cold start ~30s) |
| PostgreSQL | 1GB storage, 90-day expiry |
| Redis | 25MB RAM |
| Private services | Always on (no spin-down) |

For production use, upgrade to Render's **Starter** plan ($7/mo per service).

---

## Environment Variables Reference

### API Gateway (`gnwaas-api`)

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | `production` or `development` | `production` |
| `DEV_MODE` | Enable `/auth/dev-login` quick login | `true` |
| `DB_HOST` | PostgreSQL host (auto-set by Render) | — |
| `DB_NAME` | Database name | `gnwaas` |
| `DB_USER` | App DB user | `gnwaas_app` |
| `DB_PASSWORD` | App DB password | **Set manually** |
| `REDIS_HOST` | Redis host (auto-set by Render) | — |
| `REDIS_PASSWORD` | Redis password | **Set manually** |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allowed origins | **Set after Vercel deploy** |
| `GRA_SANDBOX_MODE` | Use GRA sandbox API | `true` |
| `SENTINEL_SERVICE_URL` | Internal sentinel URL | Auto-set |
| `TARIFF_SERVICE_URL` | Internal tariff engine URL | Auto-set |

### Frontend Portals (Vercel)

| Variable | Description |
|----------|-------------|
| `VITE_API_URL` | Full URL of Render API gateway |
| `VITE_DEV_MODE` | `false` for production (disables MSW mock) |
