from __future__ import annotations

import unittest
from datetime import datetime, timezone
from unittest.mock import patch
from urllib.parse import urlencode

import pathsetup  # noqa: F401

try:
    import jinja2  # noqa: F401

    HAVE_JINJA = True
except ImportError:  # local dev without the runtime requirements
    HAVE_JINJA = False

try:
    import starlette  # noqa: F401

    HAVE_STARLETTE = True
except ImportError:
    HAVE_STARLETTE = False

from invoices.core.http import HttpApp, Request
from invoices.core.page import Page
from invoices.core.service import InvoiceService
from invoices.core.store import MemoryStore


class FakeURL:
    def __init__(self, path: str) -> None:
        self.path = path


class FakeStarletteRequest:
    def __init__(self, method: str, path: str, *, headers: dict | None = None) -> None:
        self.method = method
        self.url = FakeURL(path)
        self.query_params: dict[str, str] = {}
        self.headers = {"x-litefunction-name": "invoices", **(headers or {})}
        self._payload = b""

    async def body(self) -> bytes:
        return self._payload


def form(payload: dict[str, str | list[str]]) -> bytes:
    items: list[tuple[str, str]] = []
    for key, value in payload.items():
        if isinstance(value, list):
            items.extend((key, item) for item in value)
        else:
            items.append((key, value))
    return urlencode(items).encode("utf-8")


@unittest.skipUnless(HAVE_JINJA, "jinja2 is required for the server-rendered UI")
class UITest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        clock = lambda: datetime(2026, 3, 2, 9, 0, tzinfo=timezone.utc)  # noqa: E731
        self.service = InvoiceService(MemoryStore(), clock=clock)
        self.app = HttpApp(self.service)
        self.env = patch.dict("os.environ", {"INVOICES_UI_BASE": ""})
        self.env.start()
        self.addCleanup(self.env.stop)

    async def call(
        self,
        method: str,
        path: str,
        *,
        payload: bytes | None = None,
        query: dict | None = None,
        headers: dict | None = None,
    ):
        return await self.app.handle(
            Request(
                method=method,
                path=path,
                query=query or {},
                headers={key.lower(): value for key, value in (headers or {}).items()},
                body=payload or b"",
            )
        )

    async def create_customer(self, name: str = "Acme Corp", **extra: str) -> str:
        data = {"name": name, "email": "billing@acme.test", "currency": "USD", "line1": "1 Main St", **extra}
        page = await self.call("POST", "/ui/customers", payload=form(data))
        self.assertIsInstance(page, Page)
        self.assertEqual(page.status, 303)
        return page.location.rsplit("/", 1)[-1]

    async def create_invoice(self, customer_id: str, **extra: str) -> str:
        data = {
            "customer_id": customer_id,
            "issue_date": "2026-03-02",
            "due_date": "2026-03-16",
            "line_description": ["Consulting", "Design"],
            "line_quantity": ["2", "1.5"],
            "line_unit_price": ["100.00", "80.00"],
            "line_tax": ["8.25", ""],
            "line_discount": ["", "10"],
            **extra,
        }
        page = await self.call("POST", "/ui/invoices", payload=form(data))
        self.assertIsInstance(page, Page)
        self.assertEqual(page.status, 303, page.body)
        return page.location.rsplit("/", 1)[-1]

    def assert_html(self, page: Page, *needles: str) -> None:
        self.assertIsInstance(page, Page, page)
        self.assertTrue(page.content_type.startswith("text/html"), page.content_type)
        for needle in needles:
            self.assertIn(needle, page.body)

    async def test_dashboard_is_served_for_ui_and_html_root(self) -> None:
        page = await self.call("GET", "/ui")
        self.assert_html(page, "Billing at a glance", "New invoice")

        root = await self.call("GET", "/", headers={"accept": "text/html,application/xhtml+xml"})
        self.assert_html(root, "Billing at a glance")

        health = await self.call("GET", "/", headers={"accept": "application/json"})
        self.assertIsInstance(health, dict)
        self.assertEqual(health["data"]["service"], "invoices")

    async def test_customer_crud_through_forms(self) -> None:
        customer_id = await self.create_customer()

        listed = await self.call("GET", "/ui/customers")
        self.assert_html(listed, "Acme Corp", "billing@acme.test")

        detail = await self.call("GET", f"/ui/customers/{customer_id}")
        self.assert_html(detail, "Acme Corp", "1 Main St", "Payment terms")

        page = await self.call(
            "POST",
            f"/ui/customers/{customer_id}",
            payload=form({"name": "Acme Holdings", "currency": "EUR", "payment_terms_days": "30"}),
        )
        self.assertEqual(page.status, 303)
        detail = await self.call("GET", f"/ui/customers/{customer_id}")
        self.assert_html(detail, "Acme Holdings", "EUR", "30 days")

    async def test_invoice_create_issue_and_pay_through_forms(self) -> None:
        customer_id = await self.create_customer()
        invoice_id = await self.create_invoice(customer_id)

        detail = await self.call("GET", f"/ui/invoices/{invoice_id}")
        self.assert_html(detail, "Draft invoice", "Consulting", "2", "100.00", "324.50")

        issued = await self.call("POST", f"/ui/invoices/{invoice_id}/issue")
        self.assertEqual(issued.status, 303)
        detail = await self.call("GET", f"/ui/invoices/{invoice_id}")
        self.assert_html(detail, "INV-2026-0001", "Open")

        paid = await self.call(
            "POST",
            f"/ui/invoices/{invoice_id}/payments",
            payload=form({"amount": "324.50", "method": "card", "reference": "TX-1"}),
        )
        self.assertEqual(paid.status, 303)
        detail = await self.call("GET", f"/ui/invoices/{invoice_id}")
        self.assert_html(detail, "Paid", "324.50", "Card", "TX-1")

        printed = await self.call("GET", f"/ui/invoices/{invoice_id}/print")
        self.assert_html(printed, "<!doctype html>", "Acme Corp", "INV-2026-0001")

    async def test_invoice_form_reports_validation_errors(self) -> None:
        page = await self.call("POST", "/ui/customers", payload=form({"name": ""}))
        self.assert_html(page, "Validation Error", "name is required")
        self.assertEqual(page.status, 422)

        customer_id = await self.create_customer()
        page = await self.call(
            "POST",
            "/ui/invoices",
            payload=form(
                {
                    "customer_id": customer_id,
                    "line_description": ["Work"],
                    "line_quantity": ["0"],
                    "line_unit_price": ["10.00"],
                }
            ),
        )
        self.assert_html(page, "greater than zero", "Work")

    async def test_ui_escapes_untrusted_content(self) -> None:
        customer_id = await self.create_customer(name="<script>alert(1)</script>")
        detail = await self.call("GET", f"/ui/customers/{customer_id}")
        self.assertNotIn("<script>alert(1)</script>", detail.body)
        self.assertIn("&lt;script&gt;alert(1)&lt;/script&gt;", detail.body)

    async def test_static_assets_and_traversal_guard(self) -> None:
        css = await self.call("GET", "/static/app.css")
        self.assertIsInstance(css, Page)
        self.assertTrue(css.content_type.startswith("text/css"))
        self.assertIn("--accent", css.as_bytes().decode("utf-8"))

        js = await self.call("GET", "/static/app.js")
        self.assertTrue(js.content_type.startswith("application/javascript"))

        missing = await self.call("GET", "/static/nope.css")
        self.assertEqual(missing.status, 404)

        traversal = await self.call("GET", "/static/../../handle.py")
        self.assertEqual(traversal.status, 404)

    async def test_links_use_the_deployed_lambda_prefix(self) -> None:
        with patch("invoices.core.ui.os.environ", {"PROJECT": "one"}):
            page = await self.call("GET", "/ui")
        self.assert_html(page, "/lambda/one/invoices/static/app.css")

    @unittest.skipUnless(HAVE_STARLETTE, "starlette is provided by the runtime")
    async def test_starlette_adapter_returns_real_responses(self) -> None:
        request = FakeStarletteRequest("GET", "/ui", headers={"accept": "text/html"})
        response = await self.app.from_starlette(request)
        self.assertEqual(response.status_code, 200)
        self.assertIn("text/html", response.headers["content-type"])
        self.assertIn(b"Billing at a glance", response.body)

        customer_id = await self.create_customer()
        request = FakeStarletteRequest(
            "GET", f"/ui/customers/{customer_id}", headers={"accept": "text/html"}
        )
        response = await self.app.from_starlette(request)
        self.assertEqual(response.status_code, 200)
        self.assertIn(b"Acme Corp", response.body)

    async def test_list_filters_and_pagination_urls(self) -> None:
        customer_id = await self.create_customer()
        for _ in range(3):
            await self.create_invoice(customer_id)

        page = await self.call("GET", "/ui/invoices", query={"status": "draft"})
        self.assert_html(page, "3 invoices", "Draft")

        page = await self.call("GET", "/ui/invoices", query={"customer_id": customer_id})
        self.assert_html(page, "3 invoices")

        page = await self.call("GET", "/ui/invoices", query={"status": "paid"})
        self.assert_html(page, "No invoices match")

        page = await self.call("GET", "/ui/customers", query={"q": "acme"})
        self.assert_html(page, "Acme Corp")
