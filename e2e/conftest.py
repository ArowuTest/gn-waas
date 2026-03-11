"""
pytest configuration for GN-WAAS E2E test suite.
Uses session-scoped browser + storage state to avoid re-logging in for every test.
"""
import time
import requests
import pytest
from playwright.sync_api import sync_playwright


API_URL = "https://gnwaas-api-gateway.onrender.com/api/v1"


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "slow: marks tests as slow (deselect with '-m \"not slow\"')"
    )


def _warmup_render():
    """Ping the Render API gateway to wake it from sleep (free tier cold start)."""
    for attempt in range(3):
        try:
            r = requests.get(f"{API_URL}/health", timeout=20)
            if r.status_code < 500:
                return True
        except Exception:
            pass
        time.sleep(3)
    return False


@pytest.fixture(scope="session", autouse=True)
def warmup_api():
    """Wake up the Render API gateway before any tests run."""
    _warmup_render()


@pytest.fixture(scope="session")
def browser_instance():
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"]
        )
        yield browser
        browser.close()


@pytest.fixture(scope="session")
def admin_storage_state(browser_instance, tmp_path_factory):
    """Log in as Super Admin once and save storage state for reuse."""
    from tests.helpers import ADMIN_URL, ADMIN_CREDS, login
    state_file = str(tmp_path_factory.mktemp("auth") / "admin_state.json")
    ctx = browser_instance.new_context(viewport={"width": 1280, "height": 800})
    pg = ctx.new_page()
    pg.set_default_timeout(30000)
    login(pg, ADMIN_URL, ADMIN_CREDS["super_admin"]["email"],
          ADMIN_CREDS["super_admin"]["password"])
    ctx.storage_state(path=state_file)
    ctx.close()
    return state_file


@pytest.fixture(scope="session")
def gwl_storage_state(browser_instance, tmp_path_factory):
    """Log in as GWL Manager once and save storage state for reuse."""
    from tests.helpers import GWL_URL, GWL_CREDS, login
    state_file = str(tmp_path_factory.mktemp("auth") / "gwl_state.json")
    ctx = browser_instance.new_context(viewport={"width": 1280, "height": 800})
    pg = ctx.new_page()
    pg.set_default_timeout(30000)
    login(pg, GWL_URL, GWL_CREDS["gwl_manager"]["email"],
          GWL_CREDS["gwl_manager"]["password"])
    ctx.storage_state(path=state_file)
    ctx.close()
    return state_file


@pytest.fixture(scope="session")
def authority_storage_state(browser_instance, tmp_path_factory):
    """Log in as GRA Officer once and save storage state for reuse."""
    from tests.helpers import AUTHORITY_URL, AUTHORITY_CREDS, login
    state_file = str(tmp_path_factory.mktemp("auth") / "authority_state.json")
    ctx = browser_instance.new_context(viewport={"width": 1280, "height": 800})
    pg = ctx.new_page()
    pg.set_default_timeout(30000)
    login(pg, AUTHORITY_URL, AUTHORITY_CREDS["gra_officer"]["email"],
          AUTHORITY_CREDS["gra_officer"]["password"])
    ctx.storage_state(path=state_file)
    ctx.close()
    return state_file


@pytest.fixture
def page(browser_instance):
    """Fresh unauthenticated page for auth tests."""
    ctx = browser_instance.new_context(
        viewport={"width": 1280, "height": 800},
        ignore_https_errors=True,
    )
    pg = ctx.new_page()
    pg.set_default_timeout(25000)
    yield pg
    ctx.close()


@pytest.fixture
def admin_page(browser_instance, admin_storage_state):
    """Page pre-authenticated as Super Admin."""
    ctx = browser_instance.new_context(
        viewport={"width": 1280, "height": 800},
        storage_state=admin_storage_state,
    )
    pg = ctx.new_page()
    pg.set_default_timeout(25000)
    yield pg
    ctx.close()


@pytest.fixture
def gwl_page(browser_instance, gwl_storage_state):
    """Page pre-authenticated as GWL Manager."""
    ctx = browser_instance.new_context(
        viewport={"width": 1280, "height": 800},
        storage_state=gwl_storage_state,
    )
    pg = ctx.new_page()
    pg.set_default_timeout(25000)
    yield pg
    ctx.close()


@pytest.fixture
def authority_page(browser_instance, authority_storage_state):
    """Page pre-authenticated as GRA Officer."""
    ctx = browser_instance.new_context(
        viewport={"width": 1280, "height": 800},
        storage_state=authority_storage_state,
    )
    pg = ctx.new_page()
    pg.set_default_timeout(25000)
    yield pg
    ctx.close()


# Ensure tests/ directory is on sys.path for helper imports
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "tests"))
