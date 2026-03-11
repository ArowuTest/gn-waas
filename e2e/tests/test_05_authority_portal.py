"""
Authority Portal — S6: Audit Workflow & GRA Compliance
Tests the regulatory oversight portal used by GRA Officers, MOF Auditors,
Field Supervisors, and Minister-level viewers.
"""
import pytest
from playwright.sync_api import Page
from helpers import AUTHORITY_URL, AUTHORITY_CREDS, login


def test_authority_login_succeeds(authority_page: Page):
    """GRA Officer is authenticated (not on login page)."""
    authority_page.goto(f"{AUTHORITY_URL}/district", wait_until="networkidle")
    assert "/login" not in authority_page.url, f"Still on login page: {authority_page.url}"


def test_authority_dashboard_loads(authority_page: Page):
    """Authority portal dashboard loads with district/audit data."""
    authority_page.goto(f"{AUTHORITY_URL}/district", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "District", "NRW", "Audit", "GRA", "Revenue", "Account", "GHS"
    ]), f"Authority dashboard content not found. Snippet: {content[:400]}"


def test_authority_dashboard_no_api_errors(authority_page: Page):
    """Authority dashboard loads without 500 API errors."""
    api_errors = []
    authority_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    authority_page.goto(f"{AUTHORITY_URL}/district", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on authority dashboard: {api_errors}"


def test_authority_nrw_summary_loads(authority_page: Page):
    """NRW Summary page loads with water balance data."""
    authority_page.goto(f"{AUTHORITY_URL}/nrw", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "NRW", "Non-Revenue", "Water Balance", "Loss", "m³", "District"
    ]), f"NRW summary content not found. Snippet: {content[:400]}"


def test_authority_nrw_no_api_errors(authority_page: Page):
    """NRW summary page fetches data without 500 errors."""
    api_errors = []
    authority_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    authority_page.goto(f"{AUTHORITY_URL}/nrw", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on NRW summary: {api_errors}"


def test_authority_anomaly_flags_page_loads(authority_page: Page):
    """Anomaly Flags page loads for GRA Officer."""
    authority_page.goto(f"{AUTHORITY_URL}/anomaly-flags", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Anomal", "Flag", "Sentinel", "Variance", "Alert", "Fraud", "Status"
    ]), f"Anomaly flags content not found. Snippet: {content[:400]}"


def test_authority_audit_events_page_loads(authority_page: Page):
    """Audit Events page loads."""
    authority_page.goto(f"{AUTHORITY_URL}/audit-events", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Audit", "Event", "GRA", "Compliance", "Status", "District"
    ]), f"Audit events content not found. Snippet: {content[:400]}"


def test_authority_audit_events_no_api_errors(authority_page: Page):
    """Audit events page fetches data without 500 errors."""
    api_errors = []
    authority_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    authority_page.goto(f"{AUTHORITY_URL}/audit-events", wait_until="networkidle")
    assert len(api_errors) == 0, f"API errors on audit events: {api_errors}"


def test_authority_reporting_page_loads(authority_page: Page):
    """Reporting page loads (previously broken — was calling wrong API path)."""
    authority_page.goto(f"{AUTHORITY_URL}/reporting", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Report", "Monthly", "GRA", "Compliance", "Download", "CSV", "Export"
    ]), f"Reporting page content not found. Snippet: {content[:400]}"


def test_authority_reporting_no_api_errors(authority_page: Page):
    """Reporting page fetches /gwl/reports/monthly without 500 errors."""
    api_errors = []
    authority_page.on("response", lambda r: api_errors.append(f"{r.status} {r.url}")
            if r.status >= 500 and "onrender.com" in r.url else None)
    authority_page.goto(f"{AUTHORITY_URL}/reporting", wait_until="networkidle")
    assert len(api_errors) == 0, \
        f"API errors on reporting page (check /gwl/reports/monthly path): {api_errors}"


def test_authority_account_search_loads(authority_page: Page):
    """Account Search page loads."""
    authority_page.goto(f"{AUTHORITY_URL}/accounts", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Account", "Search", "Customer", "Meter", "District", "ID"
    ]), f"Account search content not found. Snippet: {content[:400]}"


def test_authority_my_jobs_page_loads(authority_page: Page):
    """My Jobs page loads for field supervisor."""
    authority_page.goto(f"{AUTHORITY_URL}/my-jobs", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Job", "Field", "Assignment", "Status", "Queue", "Officer"
    ]), f"My jobs content not found. Snippet: {content[:400]}"


def test_authority_field_officers_page_loads(authority_page: Page):
    """Field Officers management page loads."""
    authority_page.goto(f"{AUTHORITY_URL}/field-officers", wait_until="networkidle")
    content = authority_page.content()
    assert any(kw in content for kw in [
        "Officer", "Field", "Staff", "Assignment", "District", "Status"
    ]), f"Field officers content not found. Snippet: {content[:400]}"


def test_authority_mof_auditor_login(page: Page):
    """MOF Auditor can log in to authority portal."""
    creds = AUTHORITY_CREDS["mof_auditor"]
    login(page, AUTHORITY_URL, creds["email"], creds["password"])
    assert "/login" not in page.url, f"MOF Auditor login failed: {page.url}"


def test_authority_field_supervisor_login(page: Page):
    """Field Supervisor can log in to authority portal."""
    creds = AUTHORITY_CREDS["field_supervisor"]
    login(page, AUTHORITY_URL, creds["email"], creds["password"])
    assert "/login" not in page.url, f"Field Supervisor login failed: {page.url}"
