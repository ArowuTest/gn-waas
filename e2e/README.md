# GN-WAAS Production E2E Test Suite

End-to-end tests that run against the **live deployed system** — not mocks, not unit tests.

## Deployed URLs Under Test

| Service | URL |
|---------|-----|
| Admin Portal | https://gnwaas-admin.vercel.app |
| GWL Portal | https://gnwaas-gwl.vercel.app |
| Authority Portal | https://gnwaas-authority.vercel.app |
| Landing Page | https://gnwaas-landing.vercel.app |
| API Gateway | https://gnwaas-api-gateway.onrender.com |

## Test Coverage

| File | Scenarios | What It Tests |
|------|-----------|---------------|
| `test_01_landing.py` | Landing | Public page loads, NRW metric, portal links |
| `test_02_admin_auth.py` | Auth | Dev-mode login for all admin roles, logout |
| `test_03_admin_portal.py` | S1,S2,S3,S4,S6,S7,S8,S9 | All admin portal pages, API error detection |
| `test_04_gwl_portal.py` | S5 | Case queue, case detail, credits, reports |
| `test_05_authority_portal.py` | S6 | GRA compliance, audit events, reporting |
| `test_06_business_logic.py` | All | Real data flows, PURC rates, RBAC, bug regressions |

## Running the Tests

```bash
cd e2e
pip install playwright pytest pytest-timeout
playwright install chromium

# Run all tests
pytest

# Run a specific portal
pytest tests/test_03_admin_portal.py -v

# Run only business logic tests
pytest tests/test_06_business_logic.py -v

# Run with HTML report
pytest --html=report.html --self-contained-html
```

## Credentials (Dev Mode)

All portals have `VITE_DEV_MODE=true` — quick-login buttons are visible on the login page.

| Portal | Role | Email | Password |
|--------|------|-------|----------|
| Admin | Super Admin | superadmin@gnwaas.gov.gh | Admin@GN2026! |
| Admin | MOF Auditor | auditor1@mof.gov.gh | MoF@Audit2026! |
| GWL | GWL Manager | manager.accrawest@gwl.com.gh | GWL@Manager2026! |
| GWL | GWL Analyst | analyst1@gwl.com.gh | GWL@Analyst2026! |
| Authority | GRA Officer | graofficer1@gra.gov.gh | GRA@Officer2026! |
| Authority | Field Supervisor | supervisor.accra@gnwaas.gov.gh | Field@Super2026! |
