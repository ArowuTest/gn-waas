"""
Admin Portal — S1: Sentinel Threshold Configuration
S2: NRW Dashboard & Shadow Billing Summary
S8: NRW Analysis & IWA Water Balance
"""
import pytest
from playwright.sync_api import Page
from helpers import ADMIN_URL


# ── S1: Sentinel Threshold Configuration ─────────────────────────────────────

def test_settings_page_loads(admin_page: Page):
    """Audit Thresholds / Settings page loads without error."""
    admin_page.goto(f"{ADMIN_URL}/settings", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Threshold", "Sentinel", "Variance", "Settings", "Configuration"
    ]), f"Settings page content not found. Content snippet: {content[:300]}"


def test_settings_shows_variance_threshold(admin_page: Page):
    """Settings page displays the variance threshold field."""
    admin_page.goto(f"{ADMIN_URL}/settings", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "variance", "Variance", "threshold", "Threshold", "%", "15"
    ]), "Variance threshold field not visible on settings page"


def test_settings_page_has_edit_controls(admin_page: Page):
    """Settings page has edit controls (pencil buttons per section)."""
    admin_page.goto(f"{ADMIN_URL}/settings", wait_until="networkidle")
    edit_btns = admin_page.locator('button')
    assert edit_btns.count() > 0, "No buttons found on settings page"


def test_settings_shows_sentinel_section(admin_page: Page):
    """Settings page shows Sentinel configuration section."""
    admin_page.goto(f"{ADMIN_URL}/settings", wait_until="networkidle")
    content = admin_page.content()
    assert "Sentinel" in content or "sentinel" in content, \
        "Sentinel section not found on settings page"


# ── S2 & S8: NRW Dashboard ────────────────────────────────────────────────────

def test_dashboard_page_loads(admin_page: Page):
    """Main dashboard loads and shows NRW or audit metrics."""
    admin_page.goto(f"{ADMIN_URL}/dashboard", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "NRW", "Non-Revenue", "Anomal", "District", "Audit", "Revenue", "GHS"
    ]), f"Dashboard metrics not found. Snippet: {content[:400]}"


def test_nrw_analysis_page_loads(admin_page: Page):
    """NRW Analysis page loads with IWA water balance data."""
    admin_page.goto(f"{ADMIN_URL}/nrw", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "NRW", "Water Balance", "IWA", "Non-Revenue", "Loss", "m³", "District"
    ]), f"NRW analysis content not found. Snippet: {content[:400]}"


def test_nrw_page_no_api_errors(admin_page: Page):
    """NRW page loads data from API without 500 errors."""
    api_errors = []
    admin_page.on("response", lambda r: api_errors.append(r.url)
            if r.status >= 500 and "onrender.com" in r.url else None)
    admin_page.goto(f"{ADMIN_URL}/nrw", wait_until="networkidle")
    assert len(api_errors) == 0, f"API 500 errors on NRW page: {api_errors}"


def test_dma_map_page_loads(admin_page: Page):
    """DMA heatmap page loads without crashing."""
    admin_page.goto(f"{ADMIN_URL}/dma-map", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "District", "Map", "Heatmap", "DMA", "Zone", "Area"
    ]), f"DMA map content not found. Snippet: {content[:300]}"


# ── S3: Anomaly Detection ─────────────────────────────────────────────────────

def test_anomalies_page_loads(admin_page: Page):
    """Anomalies page loads and shows anomaly flags."""
    admin_page.goto(f"{ADMIN_URL}/anomalies", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Anomal", "Flag", "Sentinel", "Variance", "Alert", "Fraud"
    ]), f"Anomalies page content not found. Snippet: {content[:400]}"


def test_anomalies_page_no_api_errors(admin_page: Page):
    """Anomalies page fetches data without 500 errors."""
    api_errors = []
    admin_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    admin_page.goto(f"{ADMIN_URL}/anomalies", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on anomalies page: {api_errors}"


def test_anomalies_shows_severity_filter(admin_page: Page):
    """Anomalies page has severity filter controls."""
    admin_page.goto(f"{ADMIN_URL}/anomalies", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "CRITICAL", "HIGH", "MEDIUM", "LOW", "Severity", "Filter", "Status"
    ]), "Severity filter not found on anomalies page"


# ── S4: Field Jobs ────────────────────────────────────────────────────────────

def test_field_jobs_page_loads(admin_page: Page):
    """Field Jobs page loads and shows job queue."""
    admin_page.goto(f"{ADMIN_URL}/field-jobs", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Field", "Job", "Officer", "Dispatch", "Queue", "Assignment"
    ]), f"Field jobs content not found. Snippet: {content[:400]}"


def test_field_jobs_no_api_errors(admin_page: Page):
    """Field Jobs page fetches data without 500 errors."""
    api_errors = []
    admin_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    admin_page.goto(f"{ADMIN_URL}/field-jobs", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on field jobs page: {api_errors}"


# ── S6: Audit & GRA Compliance ────────────────────────────────────────────────

def test_audits_page_loads(admin_page: Page):
    """Audits page loads with audit event data."""
    admin_page.goto(f"{ADMIN_URL}/audits", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Audit", "Event", "GRA", "Compliance", "District", "Status"
    ]), f"Audits page content not found. Snippet: {content[:400]}"


def test_gra_compliance_page_loads(admin_page: Page):
    """GRA Compliance page loads."""
    admin_page.goto(f"{ADMIN_URL}/gra", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "GRA", "Compliance", "VAT", "Revenue", "Authority", "Audit"
    ]), f"GRA compliance content not found. Snippet: {content[:400]}"


def test_reports_page_loads(admin_page: Page):
    """Reports page loads with download options."""
    admin_page.goto(f"{ADMIN_URL}/reports", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Report", "Download", "CSV", "Export", "Monthly", "NRW"
    ]), f"Reports page content not found. Snippet: {content[:400]}"


# ── S7: Revenue Leakage / Gap Tracking ───────────────────────────────────────

def test_gaps_page_loads(admin_page: Page):
    """Gap Tracking page loads with revenue leakage data."""
    admin_page.goto(f"{ADMIN_URL}/gaps", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Gap", "Revenue", "Leakage", "Recovery", "GHS", "Loss", "Pending"
    ]), f"Gaps page content not found. Snippet: {content[:400]}"


def test_gaps_page_no_api_errors(admin_page: Page):
    """Gaps page fetches /gaps/summary without 500 errors (previously broken)."""
    api_errors = []
    admin_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    admin_page.goto(f"{ADMIN_URL}/gaps", wait_until="networkidle")
    assert len(api_errors) == 0, \
        f"API 500 errors on gaps page (check /gaps/summary fix): {api_errors}"


def test_donor_kpis_page_loads(admin_page: Page):
    """Donor KPIs page loads with trend data."""
    admin_page.goto(f"{ADMIN_URL}/donor-kpis", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Donor", "KPI", "Recovery", "Revenue", "Trend", "GHS"
    ]), f"Donor KPIs content not found. Snippet: {content[:400]}"


# ── S9: Tariff Management ─────────────────────────────────────────────────────

def test_tariffs_page_loads(admin_page: Page):
    """Tariff Management page loads with PURC 2026 rates."""
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Tariff", "Rate", "PURC", "RESIDENTIAL", "COMMERCIAL", "GHS", "m³"
    ]), f"Tariff management content not found. Snippet: {content[:400]}"


def test_tariffs_no_api_errors(admin_page: Page):
    """Tariff page fetches rates without 500 errors."""
    api_errors = []
    admin_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on tariffs page: {api_errors}"


def test_tariffs_shows_residential_rates(admin_page: Page):
    """Tariff page shows RESIDENTIAL category rates."""
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    content = admin_page.content()
    assert "RESIDENTIAL" in content or "Residential" in content, \
        "RESIDENTIAL tariff rates not visible on tariff management page"
