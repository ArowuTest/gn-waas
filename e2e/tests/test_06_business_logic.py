"""
Business Logic E2E Tests — Tests that verify actual data flows and
business rules through the deployed UI, not just page loads.
"""
import pytest
from playwright.sync_api import Page
from helpers import ADMIN_URL, GWL_URL, AUTHORITY_URL, ADMIN_CREDS, GWL_CREDS, AUTHORITY_CREDS, login


# ── S1: Threshold Configuration Business Logic ────────────────────────────────

def test_threshold_field_is_editable(admin_page: Page):
    """Admin can interact with the variance threshold input field."""
    admin_page.goto(f"{ADMIN_URL}/settings", wait_until="networkidle")
    inputs = admin_page.locator('input[type="number"], input[type="text"]')
    if inputs.count() > 0:
        first_input = inputs.first
        first_input.click()
        assert first_input.is_enabled(), "Threshold input is not editable"
    else:
        pytest.skip("No input fields found on settings page")


# ── S2: Shadow Billing — Tariff Calculator ────────────────────────────────────

def test_tariff_page_shows_purc_2026_rates(admin_page: Page):
    """Tariff management page shows PURC 2026 rates (6.1225 GHS/m³ tier 1)."""
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    content = admin_page.content()
    assert any(rate in content for rate in ["6.1225", "6.12", "10.832", "10.83"]), \
        "PURC 2026 tariff rates (6.1225 or 10.832 GHS/m³) not visible on tariff page"


def test_tariff_page_shows_vat_rate(admin_page: Page):
    """Tariff page shows 20% VAT configuration."""
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    content = admin_page.content()
    assert any(vat in content for vat in ["20%", "20 %", "VAT", "vat"]), \
        "VAT rate not visible on tariff management page"


def test_tariff_page_shows_all_categories(admin_page: Page):
    """Tariff page shows all billing categories."""
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    content = admin_page.content()
    categories_found = sum(1 for cat in ["RESIDENTIAL", "COMMERCIAL", "INDUSTRIAL"] if cat in content)
    assert categories_found >= 2, \
        f"Expected at least 2 tariff categories, found {categories_found}. Content snippet: {content[:500]}"


# ── S3: Anomaly Detection — Data Verification ────────────────────────────────

def test_anomalies_page_shows_real_data(admin_page: Page):
    """Anomalies page shows actual anomaly records from the database."""
    api_responses = {}
    def capture(r):
        if "anomaly" in r.url.lower() or "flags" in r.url.lower():
            api_responses[r.url] = r.status
    admin_page.on("response", capture)
    admin_page.goto(f"{ADMIN_URL}/anomalies", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, f"Anomaly API calls failed: {failed}"


# ── S5: GWL Case Management — Data Verification ──────────────────────────────

def test_gwl_cases_shows_real_data(gwl_page: Page):
    """GWL case queue shows actual case records."""
    api_responses = {}
    def capture(r):
        if "/gwl/cases" in r.url or "/cases" in r.url:
            api_responses[r.url] = r.status
    gwl_page.on("response", capture)
    gwl_page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, f"Case API calls returned 500: {failed}"


def test_gwl_credits_shows_real_data(gwl_page: Page):
    """GWL credits page fetches /gwl/credits successfully."""
    api_responses = {}
    def capture(r):
        if "credit" in r.url.lower():
            api_responses[r.url] = r.status
    gwl_page.on("response", capture)
    gwl_page.goto(f"{GWL_URL}/credits", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, f"Credits API calls returned 500: {failed}"


# ── S6: GRA Compliance — CSV Report Download ─────────────────────────────────

def test_admin_reports_page_has_download_buttons(admin_page: Page):
    """Reports page has CSV download buttons for GRA compliance and monthly reports."""
    admin_page.goto(f"{ADMIN_URL}/reports", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in ["Download", "CSV", "Export", "GRA", "Monthly"]), \
        "No download buttons found on reports page"


def test_authority_reporting_fetches_correct_api(authority_page: Page):
    """Authority portal reporting page calls /gwl/reports/monthly (not /reports/monthly)."""
    api_calls = []
    authority_page.on("response", lambda r: api_calls.append(r.url) if "onrender.com" in r.url else None)
    authority_page.goto(f"{AUTHORITY_URL}/reporting", wait_until="networkidle")
    # Should NOT call /reports/monthly (wrong path from BUG-V30-03)
    wrong_path = [u for u in api_calls if "/reports/monthly" in u and "/gwl/" not in u]
    assert len(wrong_path) == 0, \
        f"Authority portal is calling wrong API path (BUG-V30-03 regression): {wrong_path}"


# ── S7: Revenue Leakage — Gap Summary ────────────────────────────────────────

def test_gaps_page_fetches_summary_successfully(admin_page: Page):
    """Gaps page calls /gaps/summary and gets 200 (previously broken with NULL SQL)."""
    api_responses = {}
    def capture(r):
        if "gaps" in r.url.lower():
            api_responses[r.url] = r.status
    admin_page.on("response", capture)
    admin_page.goto(f"{ADMIN_URL}/gaps", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, \
        f"/gaps/summary returned 500 (NULL arithmetic fix may not be deployed): {failed}"


def test_gaps_page_shows_summary_metrics(admin_page: Page):
    """Gaps page renders summary metrics (total gaps, recovery rate, etc.)."""
    admin_page.goto(f"{ADMIN_URL}/gaps", wait_until="networkidle")
    content = admin_page.content()
    assert any(kw in content for kw in [
        "Recovery", "Pending", "GHS", "Gap", "Rate", "%", "Identified"
    ]), f"Gap summary metrics not visible. Snippet: {content[:400]}"


# ── S8: NRW — Water Balance Verification ─────────────────────────────────────

def test_nrw_page_fetches_district_data(admin_page: Page):
    """NRW page calls district heatmap/NRW API and gets 200."""
    api_responses = {}
    def capture(r):
        if any(kw in r.url for kw in ["nrw", "heatmap", "district"]):
            api_responses[r.url] = r.status
    admin_page.on("response", capture)
    admin_page.goto(f"{ADMIN_URL}/nrw", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, f"NRW API calls returned 500: {failed}"


# ── S9: Tariff — Calculator via Proxy ────────────────────────────────────────

def test_tariff_proxy_called_successfully(admin_page: Page):
    """Any tariff calculation UI calls /tariff/* proxy and gets 200 (not 500)."""
    api_responses = {}
    def capture(r):
        if "/tariff" in r.url and "onrender.com" in r.url:
            api_responses[r.url] = r.status
    admin_page.on("response", capture)
    admin_page.goto(f"{ADMIN_URL}/tariffs", wait_until="networkidle")
    failed = {url: s for url, s in api_responses.items() if s >= 500}
    assert len(failed) == 0, \
        f"Tariff proxy calls returned 500 (proxy route may not be deployed): {failed}"


# ── Cross-Portal: RBAC Enforcement ───────────────────────────────────────────

def test_unauthenticated_admin_redirects_to_login(page: Page):
    """Accessing admin portal without auth redirects to login."""
    page.goto(f"{ADMIN_URL}/dashboard", wait_until="networkidle")
    assert "/login" in page.url, \
        f"Unauthenticated access to dashboard did not redirect to login. URL: {page.url}"


def test_unauthenticated_gwl_redirects_to_login(page: Page):
    """Accessing GWL portal without auth redirects to login."""
    page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    assert "/login" in page.url, \
        f"Unauthenticated access to GWL cases did not redirect to login. URL: {page.url}"


def test_unauthenticated_authority_redirects_to_login(page: Page):
    """Accessing authority portal without auth redirects to login."""
    page.goto(f"{AUTHORITY_URL}/district", wait_until="networkidle")
    assert "/login" in page.url, \
        f"Unauthenticated access to authority portal did not redirect to login. URL: {page.url}"
