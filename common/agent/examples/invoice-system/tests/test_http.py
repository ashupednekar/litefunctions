from __future__ import annotations

import json
import unittest
from datetime import datetime, timezone

import pathsetup  # noqa: F401

from invoices.core.http import HttpApp, Request
from invoices.core.service import InvoiceService
from invoices.core.store import MemoryStore


def body(payload: dict) -> bytes:
    return json.dumps(payload).encode("utf-8")


class FakeURL:
    def __init__(self, path: str) -> None:
        self.path = path


class FakeStarletteRequest:
    def __init__(
        self,
        method: str,
        path: str,
        payload: dict | None = None,
        query: dict | None = None,
        headers: dict | None = None,
    ) -> None:
        self.method = method
        self.url = FakeURL(path)
        self.query_params: dict[str, str] = query or {}
        self.headers = {"x-litefunction-name": "invoices", **(headers or {})}
        self._payload = body(payload or {})

    async def body(self) -> bytes:
        return self._payload


class HttpTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        clock = lambda: datetime(2026, 3, 2, 9, 0, tzinfo=timezone.utc)  # noqa: E731
        self.service = InvoiceService(MemoryStore(), clock=clock)
        self.app = HttpApp(self.service)

    async def call(self, method: str, path: str, payload: dict | None = None, query: dict | None = None, headers: dict | None = None):
        return await self.app.handle(
            Request(
                method=method,
                path=path,
                query=query or {},
                headers={key.lower(): value for key, value in (headers or {}).items()},
                body=body(payload) if payload is not None else b"",
            )
        )

    async def seed(self) -> tuple[str, str]:
        customer = await self.call("POST", "/customers", {"name": "Acme", "currency": "USD"})
        customer_id = customer["data"]["id"]
        invoice = await self.call(
            "POST",
            "/invoices",
            {
                "customer_id": customer_id,
                "line_items": [
                    {"description": "Work", "quantity": 2, "unit_price": "50.00", "tax_rate_bp": 1000}
                ],
            },
        )
        return customer_id, invoice["data"]["id"]

    async def test_end_to_end_http_flow(self) -> None:
        customer_id, invoice_id = await self.seed()

        issued = await self.call("POST", f"/invoices/{invoice_id}/issue")
        self.assertTrue(issued["ok"])
        self.assertEqual(issued["status"], 200)
        self.assertEqual(issued["data"]["number"], "INV-2026-0001")
        self.assertEqual(issued["data"]["totals"]["total"], "110.00")

        paid = await self.call(
            "POST",
            f"/invoices/{invoice_id}/payments",
            {"amount": "110.00", "method": "card"},
        )
        self.assertTrue(paid["ok"])
        self.assertEqual(paid["status"], 201)
        self.assertEqual(paid["data"]["amount"], "110.00")

        shown = await self.call("GET", f"/invoices/{invoice_id}")
        self.assertEqual(shown["data"]["status"], "paid")

        payments = await self.call("GET", f"/invoices/{invoice_id}/payments")
        self.assertEqual(payments["data"]["count"], 1)

        summary = await self.call("GET", "/summary")
        self.assertEqual(summary["data"]["counts"]["paid"], 1)

        rendered = await self.call("GET", f"/invoices/{invoice_id}/render")
        self.assertIn("Acme", rendered["data"]["html"])
        self.assertIn("110.00", rendered["data"]["html"])

        customers = await self.call("GET", "/customers", query={"q": "acme"})
        self.assertEqual(customers["data"]["total"], 1)
        self.assertEqual(customers["data"]["items"][0]["id"], customer_id)

    async def test_created_responses_use_201_in_the_envelope(self) -> None:
        created = await self.call("POST", "/customers", {"name": "Acme"})
        self.assertEqual(created["status"], 201)

    async def test_not_found_method_not_allowed_and_validation_envelopes(self) -> None:
        missing = await self.call("GET", "/nope")
        self.assertFalse(missing["ok"])
        self.assertEqual(missing["status"], 404)
        self.assertEqual(missing["error"]["code"], "not_found")

        wrong_method = await self.call("POST", "/summary")
        self.assertEqual(wrong_method["status"], 405)
        self.assertEqual(wrong_method["error"]["code"], "method_not_allowed")
        self.assertEqual(wrong_method["error"]["details"]["allowed"], ["GET"])

        invalid = await self.call("POST", "/invoices", {"line_items": []})
        self.assertFalse(invalid["ok"])
        self.assertEqual(invalid["status"], 422)
        self.assertEqual(invalid["error"]["code"], "validation_error")

        bad_offset = await self.call("GET", "/invoices", query={"offset": "-1"})
        self.assertEqual(bad_offset["status"], 422)

    async def test_idempotency_header_is_honoured(self) -> None:
        payload = {"name": "Acme", "currency": "USD"}
        first = await self.call("POST", "/customers", payload, headers={"Idempotency-Key": "abc"})
        second = await self.call("POST", "/customers", payload, headers={"Idempotency-Key": "abc"})
        self.assertEqual(first["data"]["id"], second["data"]["id"])

        conflict = await self.call(
            "POST", "/customers", {"name": "Other"}, headers={"Idempotency-Key": "abc"}
        )
        self.assertEqual(conflict["status"], 409)
        self.assertEqual(conflict["error"]["code"], "idempotency_key_reused")

    async def test_starlette_adapter(self) -> None:
        response = await self.app.from_starlette(
            FakeStarletteRequest("POST", "/customers", {"name": "Adapter Co"})
        )
        self.assertTrue(response["ok"])
        self.assertEqual(response["data"]["name"], "Adapter Co")

    async def test_single_endpoint_op_mode_for_the_ingestor_proxy(self) -> None:
        # The ingestor forwards only `/lambda/<project>/<name>`, so the runtime
        # sees path `/`. Operations arrive via ?op= or an "op" body field.
        health = await self.app.from_starlette(FakeStarletteRequest("GET", "/"))
        self.assertEqual(health["data"]["service"], "invoices")

        created = await self.app.from_starlette(
            FakeStarletteRequest("POST", "/", {"op": "create_customer", "name": "Cluster Co"})
        )
        self.assertTrue(created["ok"])
        customer_id = created["data"]["id"]

        invoice = await self.app.from_starlette(
            FakeStarletteRequest(
                "POST",
                "/",
                {
                    "op": "create_invoice",
                    "customer_id": customer_id,
                    "line_items": [{"description": "Work", "quantity": 1, "unit_price": "10.00"}],
                },
            )
        )
        self.assertTrue(invoice["ok"])
        invoice_id = invoice["data"]["id"]

        issued = await self.app.from_starlette(
            FakeStarletteRequest(
                "POST", "/", {"op": "issue_invoice"}, query={"id": invoice_id}
            )
        )
        self.assertEqual(issued["data"]["number"], "INV-2026-0001")

        idempotency = await self.app.from_starlette(
            FakeStarletteRequest(
                "POST",
                "/",
                {"op": "create_customer", "name": "Header Co"},
                headers={"Idempotency-Key": "hdr-1"},
            )
        )
        repeated = await self.app.from_starlette(
            FakeStarletteRequest(
                "POST",
                "/",
                {"op": "create_customer", "name": "Header Co"},
                headers={"Idempotency-Key": "hdr-1"},
            )
        )
        self.assertEqual(idempotency["data"]["id"], repeated["data"]["id"])

        listed = await self.app.from_starlette(
            FakeStarletteRequest("GET", "/", query={"op": "list_invoices", "status": "open"})
        )
        self.assertEqual(listed["data"]["total"], 1)

        missing = await self.app.from_starlette(
            FakeStarletteRequest("POST", "/", {"name": "no op here"})
        )
        self.assertEqual(missing["status"], 422)
        self.assertEqual(missing["error"]["code"], "missing_op")

    async def test_async_ops_dispatch_to_the_same_routes(self) -> None:
        created = await self.app.from_op(
            {"op": "create_customer", "name": "Op Co", "currency": "USD"}
        )
        self.assertTrue(created["ok"])
        customer_id = created["data"]["id"]

        invoice = await self.app.from_op(
            {
                "op": "createInvoice",
                "customer_id": customer_id,
                "line_items": [{"description": "Async work", "quantity": 1, "unit_price": "42.00"}],
            }
        )
        self.assertTrue(invoice["ok"])
        invoice_id = invoice["data"]["id"]

        issued = await self.app.from_op({"op": "issue_invoice", "invoice_id": invoice_id})
        self.assertEqual(issued["data"]["number"], "INV-2026-0001")

        paid = await self.app.from_op(
            {
                "op": "record_payment",
                "invoice_id": invoice_id,
                "input": {"amount": "42.00", "method": "cash"},
                "idempotency_key": "op-payment",
            }
        )
        self.assertTrue(paid["ok"])
        self.assertEqual(paid["data"]["amount"], "42.00")

        listed = await self.app.from_op({"op": "list_invoices", "status": "paid"})
        self.assertEqual(listed["data"]["total"], 1)

        unknown = await self.app.from_op({"op": "does_not_exist"})
        self.assertEqual(unknown["status"], 422)
        self.assertEqual(unknown["error"]["code"], "validation_error")

        missing_op = await self.app.from_op({"nothing": True})
        self.assertEqual(missing_op["status"], 422)
