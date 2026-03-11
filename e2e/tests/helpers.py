"""
GN-WAAS Production E2E Test Suite
Tests the live deployed system at:
  Admin Portal    : https://gnwaas-admin.vercel.app
  GWL Portal      : https://gnwaas-gwl.vercel.app
  Authority Portal: https://gnwaas-authority.vercel.app
  Landing Page    : https://gnwaas-landing.vercel.app
  API Gateway     : https://gnwaas-api-gateway.onrender.com
"""
import time

ADMIN_URL      = "https://gnwaas-admin.vercel.app"
GWL_URL        = "https://gnwaas-gwl.vercel.app"
AUTHORITY_URL  = "https://gnwaas-authority.vercel.app"
LANDING_URL    = "https://gnwaas-landing.vercel.app"
API_URL        = "https://gnwaas-api-gateway.onrender.com/api/v1"

# Dev-mode credentials (VITE_DEV_MODE=true on all portals)
ADMIN_CREDS = {
    "super_admin":  {"email": "superadmin@gnwaas.gov.gh",        "password": "Admin@GN2026!"},
    "system_admin": {"email": "sysadmin@gnwaas.gov.gh",          "password": "Admin@GN2026!"},
    "mof_auditor":  {"email": "auditor1@mof.gov.gh",             "password": "MoF@Audit2026!"},
    "gwl_manager":  {"email": "manager.accrawest@gwl.com.gh",    "password": "GWL@Manager2026!"},
}
GWL_CREDS = {
    "gwl_manager":    {"email": "manager.accrawest@gwl.com.gh",  "password": "GWL@Manager2026!"},
    "gwl_supervisor": {"email": "supervisor@gwl.com.gh",         "password": "GWL@Super2026!"},
    "gwl_analyst":    {"email": "analyst1@gwl.com.gh",           "password": "GWL@Analyst2026!"},
}
AUTHORITY_CREDS = {
    "gra_officer":      {"email": "graofficer1@gra.gov.gh",          "password": "GRA@Officer2026!"},
    "mof_auditor":      {"email": "auditor1@mof.gov.gh",             "password": "MoF@Audit2026!"},
    "field_supervisor": {"email": "supervisor.accra@gnwaas.gov.gh",  "password": "Field@Super2026!"},
}


def login(page, portal_url: str, email: str, password: str, retries: int = 2):
    """Navigate to portal login page and sign in. Retries on timeout (cold Render start)."""
    for attempt in range(retries + 1):
        try:
            page.goto(f"{portal_url}/login", wait_until="networkidle", timeout=20000)
            page.fill('input[type="email"], input[name="email"]', email)
            page.fill('input[type="password"], input[name="password"]', password)
            page.click('button[type="submit"]')
            page.wait_for_url(lambda url: "/login" not in url, timeout=30000)
            return  # success
        except Exception as e:
            if attempt < retries:
                time.sleep(5)  # wait for Render cold start
                continue
            raise
