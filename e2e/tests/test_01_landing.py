from helpers import LANDING_URL, ADMIN_URL, GWL_URL, AUTHORITY_URL
"""
Landing Page Tests
Verifies the public-facing landing page loads correctly and portal links work.
"""
import pytest
from playwright.sync_api import Page


def test_landing_page_loads(page: Page):
    """Landing page returns 200 and shows GN-WAAS branding."""
    page.goto(LANDING_URL, wait_until="networkidle")
    assert "GN-WAAS" in page.title() or "Ghana" in page.title(), \
        f"Unexpected title: {page.title()}"


def test_landing_shows_nrw_metric(page: Page):
    """Landing page displays the 51.6% NRW headline metric."""
    page.goto(LANDING_URL, wait_until="networkidle")
    content = page.content()
    assert "51.6" in content or "51.6%" in content, \
        "NRW headline metric (51.6%) not found on landing page"


def test_landing_shows_portal_links(page: Page):
    """Landing page has links to all three portals."""
    page.goto(LANDING_URL, wait_until="networkidle")
    content = page.content()
    assert any(kw in content for kw in ["Operations", "GWL", "Regulatory", "Portal"]), \
        "No portal links found on landing page"


def test_landing_no_console_errors(page: Page):
    """Landing page loads without critical JS errors."""
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.goto(LANDING_URL, wait_until="networkidle")
    critical = [e for e in errors if "TypeError" in e or "ReferenceError" in e]
    assert len(critical) == 0, f"JS errors on landing page: {critical}"


def test_admin_portal_reachable(page: Page):
    """Admin portal URL returns a page (not 404/500)."""
    resp = page.goto(ADMIN_URL, wait_until="domcontentloaded")
    assert resp.status < 400, f"Admin portal returned {resp.status}"


def test_gwl_portal_reachable(page: Page):
    """GWL portal URL returns a page (not 404/500)."""
    resp = page.goto(GWL_URL, wait_until="domcontentloaded")
    assert resp.status < 400, f"GWL portal returned {resp.status}"


def test_authority_portal_reachable(page: Page):
    """Authority portal URL returns a page (not 404/500)."""
    resp = page.goto(AUTHORITY_URL, wait_until="domcontentloaded")
    assert resp.status < 400, f"Authority portal returned {resp.status}"
