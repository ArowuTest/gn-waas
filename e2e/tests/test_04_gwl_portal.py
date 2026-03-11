"""
GWL Portal — S5: Case Management
Tests the GWL (Ghana Water Limited) case management portal.
"""
import pytest
from playwright.sync_api import Page
from helpers import GWL_URL, GWL_CREDS, login


def test_gwl_login_succeeds(gwl_page: Page):
    """GWL Manager is authenticated (not on login page)."""
    gwl_page.goto(f"{GWL_URL}/", wait_until="networkidle")
    assert "/login" not in gwl_page.url, f"Still on login page: {gwl_page.url}"


def test_gwl_dashboard_loads(gwl_page: Page):
    """GWL dashboard loads with case metrics."""
    gwl_page.goto(f"{GWL_URL}/", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Case", "GWL", "Dashboard", "Billing", "Underbilling", "Credit", "GHS"
    ]), f"GWL dashboard content not found. Snippet: {content[:400]}"


def test_gwl_dashboard_no_api_errors(gwl_page: Page):
    """GWL dashboard loads without 500 API errors."""
    api_errors = []
    gwl_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    gwl_page.goto(f"{GWL_URL}/", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on GWL dashboard: {api_errors}"


def test_gwl_case_queue_loads(gwl_page: Page):
    """Case Queue page loads with case list."""
    gwl_page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Case", "Queue", "Status", "Account", "District", "GHS", "Billing"
    ]), f"Case queue content not found. Snippet: {content[:400]}"


def test_gwl_case_queue_no_api_errors(gwl_page: Page):
    """Case queue fetches /gwl/cases without 500 errors."""
    api_errors = []
    gwl_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    gwl_page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on case queue: {api_errors}"


def test_gwl_case_queue_shows_status_filters(gwl_page: Page):
    """Case queue has status filter controls."""
    gwl_page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "OPEN", "UNDER_INVESTIGATION", "RESOLVED", "Filter", "Status", "All"
    ]), "Status filter not found on case queue page"


def test_gwl_case_detail_navigable(gwl_page: Page):
    """Clicking a case row navigates to case detail page."""
    gwl_page.goto(f"{GWL_URL}/cases", wait_until="networkidle")
    case_row = gwl_page.locator("table tbody tr, [data-testid='case-row'], .case-item").first
    if case_row.count() > 0:
        case_row.click()
        gwl_page.wait_for_load_state("networkidle")
        assert "/cases/" in gwl_page.url or "case" in gwl_page.url.lower(), \
            f"Did not navigate to case detail. URL: {gwl_page.url}"
    else:
        pytest.skip("No case rows found — database may be empty")


def test_gwl_underbilling_page_loads(gwl_page: Page):
    """Underbilling page loads."""
    gwl_page.goto(f"{GWL_URL}/underbilling", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Underbilling", "Under", "Billing", "Account", "GHS", "Variance"
    ]), f"Underbilling content not found. Snippet: {content[:400]}"


def test_gwl_overbilling_page_loads(gwl_page: Page):
    """Overbilling page loads."""
    gwl_page.goto(f"{GWL_URL}/overbilling", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Overbilling", "Over", "Billing", "Account", "GHS", "Credit"
    ]), f"Overbilling content not found. Snippet: {content[:400]}"


def test_gwl_credits_page_loads(gwl_page: Page):
    """Credit Requests page loads."""
    gwl_page.goto(f"{GWL_URL}/credits", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Credit", "Request", "Account", "GHS", "Status", "Refund"
    ]), f"Credits page content not found. Snippet: {content[:400]}"


def test_gwl_credits_no_api_errors(gwl_page: Page):
    """Credits page fetches /gwl/credits without 500 errors."""
    api_errors = []
    gwl_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    gwl_page.goto(f"{GWL_URL}/credits", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on credits page: {api_errors}"


def test_gwl_monthly_reports_page_loads(gwl_page: Page):
    """Monthly Reports page loads with download option."""
    gwl_page.goto(f"{GWL_URL}/reports", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Report", "Monthly", "Download", "CSV", "Export", "Period"
    ]), f"Monthly reports content not found. Snippet: {content[:400]}"


def test_gwl_field_assignments_page_loads(gwl_page: Page):
    """Field Assignments page loads."""
    gwl_page.goto(f"{GWL_URL}/field-assignments", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Field", "Assignment", "Officer", "Job", "Dispatch", "Status"
    ]), f"Field assignments content not found. Snippet: {content[:400]}"


def test_gwl_misclassification_page_loads(gwl_page: Page):
    """Misclassification page loads."""
    gwl_page.goto(f"{GWL_URL}/misclassification", wait_until="networkidle")
    content = gwl_page.content()
    assert any(kw in content for kw in [
        "Misclassif", "Category", "RESIDENTIAL", "COMMERCIAL", "Account"
    ]), f"Misclassification content not found. Snippet: {content[:400]}"


def test_gwl_analyst_can_login(page: Page):
    """GWL Analyst role can log in to GWL portal."""
    creds = GWL_CREDS["gwl_analyst"]
    login(page, GWL_URL, creds["email"], creds["password"])
    assert "/login" not in page.url, f"GWL Analyst login failed: {page.url}"
