from __future__ import annotations

import unittest
from datetime import datetime, timedelta, timezone
from decimal import Decimal

import pathsetup  # noqa: F401  (adds the function package to sys.path)

from invoices.core.errors import ConflictError, NotFoundError, ValidationError
from invoices.core.models import ACTIVE, DRAFT, OPEN, PAID, PARTIALLY_PAID, VOID
from invoices.core.service import InvoiceService
from invoices.core.store import MemoryStore


class Clock:
    def __init__(self) -> None:
        self.now = datetime(2026, 1, 15, 10, 0, tzinfo=timezone.utc)

    def __call__(self) -> datetime:
        return self.now

    def advance(self, days: int) -> None:
        self.now = self.now + timedelta(days=days)


INVOICE_PAYLOAD = {
    "line_items": [
        {
            "description": "Consulting",
            "quantity": 2,
            "unit_price": "100.00",
            "tax_rate_bp": 825,
        },
        {
            "description": "Design",
            "quantity": "1.5",
            "unit_price": "80.00",
            "discount_bp": 1000,
        },
    ],
    "notes": "Thanks for the work.",
}


class ServiceTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.clock = Clock()
        self.events: list[tuple[str, dict]] = []

        async def publisher(event: str, payload: dict) -> None:
            self.events.append((event, payload))

        self.store = MemoryStore()
        self.service = InvoiceService(self.store, clock=self.clock, publisher=publisher)

    async def customer(self, **overrides) -> dict:
        payload = {
            "name": "Acme Corp",
            "email": "billing@acme.test",
            "currency": "USD",
            "payment_terms_days": 14,
            "address": {"line1": "1 Market St", "city": "San Francisco", "country": "US"},
        }
        payload.update(overrides)
        return await self.service.create_customer(payload)

    async def invoice(self, customer_id: str | None = None, **overrides) -> dict:
        customer_id = customer_id or (await self.customer())["id"]
        payload = {"customer_id": customer_id, **INVOICE_PAYLOAD}
        payload.update(overrides)
        return await self.service.create_invoice(payload)

    # ------------------------------------------------------------------
    # Money and drafts
    # ------------------------------------------------------------------

    async def test_totals_are_exact_and_presented_as_strings(self) -> None:
        invoice = await self.invoice()

        self.assertEqual(invoice["status"], DRAFT)
        self.assertEqual(invoice["currency"], "USD")
        self.assertEqual(invoice["line_items"][0]["amount"], "200.00")
        self.assertEqual(invoice["line_items"][0]["tax"], "16.50")
        self.assertEqual(invoice["line_items"][1]["amount"], "120.00")
        self.assertEqual(invoice["line_items"][1]["discount"], "12.00")
        self.assertEqual(invoice["line_items"][1]["total"], "108.00")
        self.assertEqual(
            invoice["totals"],
            {
                "subtotal": "320.00",
                "discount": "12.00",
                "tax": "16.50",
                "total": "324.50",
                "amount_paid": "0.00",
                "balance_due": "324.50",
            },
        )

    async def test_update_draft_recomputes_totals_and_rejects_currency_change(self) -> None:
        invoice = await self.invoice()
        updated = await self.service.update_invoice(
            invoice["id"],
            {
                "line_items": [
                    {"description": "Fixed fee", "quantity": 1, "unit_price": "1000.00"}
                ],
                "notes": "Revised scope",
            },
        )
        self.assertEqual(updated["totals"]["total"], "1000.00")
        self.assertEqual(updated["notes"], "Revised scope")

        with self.assertRaises(ValidationError):
            await self.service.update_invoice(invoice["id"], {"currency": "EUR"})

    async def test_issue_assigns_sequential_numbers_and_snapshots_customer(self) -> None:
        customer = await self.customer()
        first = await self.invoice(customer["id"])
        second = await self.invoice(customer["id"])

        issued_first = await self.service.issue_invoice(first["id"])
        issued_second = await self.service.issue_invoice(second["id"])

        self.assertEqual(issued_first["number"], "INV-2026-0001")
        self.assertEqual(issued_second["number"], "INV-2026-0002")
        self.assertEqual(issued_first["status"], OPEN)
        self.assertEqual(issued_first["issue_date"], "2026-01-15")
        self.assertEqual(issued_first["due_date"], "2026-01-29")
        self.assertEqual(issued_first["customer"]["name"], "Acme Corp")
        self.assertIsNotNone(issued_first["issued_at"])

    async def test_issue_requires_line_items(self) -> None:
        invoice = await self.invoice(line_items=[])
        with self.assertRaises(ValidationError):
            await self.service.issue_invoice(invoice["id"])

    # ------------------------------------------------------------------
    # Payments
    # ------------------------------------------------------------------

    async def test_payment_lifecycle_partial_then_paid(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])

        partial = await self.service.record_payment(
            invoice["id"], {"amount": "100.00", "method": "bank_transfer", "reference": "TX-1"}
        )
        self.assertEqual(partial["amount"], "100.00")

        after_partial = await self.service.get_invoice(invoice["id"])
        self.assertEqual(after_partial["status"], PARTIALLY_PAID)
        self.assertEqual(after_partial["totals"]["balance_due"], "224.50")
        self.assertIn("payment.recorded", [event for event, _ in self.events])

        await self.service.record_payment(invoice["id"], {"amount": "224.50", "method": "card"})
        done = await self.service.get_invoice(invoice["id"])
        self.assertEqual(done["status"], PAID)
        self.assertEqual(done["totals"]["balance_due"], "0.00")
        self.assertIn("invoice.paid", [event for event, _ in self.events])

        with self.assertRaises(ValidationError):
            await self.service.record_payment(invoice["id"], {"amount": "1.00"})

    async def test_overpayment_and_currency_mismatch_are_rejected(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])

        with self.assertRaises(ValidationError) as ctx:
            await self.service.record_payment(invoice["id"], {"amount": "325.00"})
        self.assertEqual(ctx.exception.code, "validation_error")

        with self.assertRaises(ValidationError):
            await self.service.record_payment(
                invoice["id"], {"amount": "10.00", "currency": "EUR"}
            )

        with self.assertRaises(ValidationError):
            await self.service.record_payment(invoice["id"], {"amount": "0"})

    async def test_payment_idempotency_returns_the_same_payment(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])

        first = await self.service.record_payment(
            invoice["id"], {"amount": "10.00"}, idempotency_key="pay-once"
        )
        second = await self.service.record_payment(
            invoice["id"], {"amount": "10.00"}, idempotency_key="pay-once"
        )
        self.assertEqual(first["id"], second["id"])

        payments = await self.service.list_payments(invoice["id"])
        self.assertEqual(payments["count"], 1)

        with self.assertRaises(ConflictError) as ctx:
            await self.service.record_payment(
                invoice["id"], {"amount": "11.00"}, idempotency_key="pay-once"
            )
        self.assertEqual(ctx.exception.code, "idempotency_key_reused")

    async def test_void_payment_reopens_the_invoice(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])
        payment = await self.service.record_payment(invoice["id"], {"amount": "324.50"})
        self.assertEqual((await self.service.get_invoice(invoice["id"]))["status"], PAID)

        result = await self.service.void_payment(
            invoice["id"], payment["id"], {"reason": "chargeback"}
        )
        self.assertEqual(result["invoice"]["status"], OPEN)
        self.assertEqual(result["invoice"]["totals"]["balance_due"], "324.50")
        self.assertIsNotNone(result["payment"]["voided_at"])

        with self.assertRaises(ConflictError):
            await self.service.void_payment(invoice["id"], payment["id"])

    # ------------------------------------------------------------------
    # Voiding and deletion
    # ------------------------------------------------------------------

    async def test_void_invoice_rules(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])

        voided = await self.service.void_invoice(invoice["id"], {"reason": "duplicate"})
        self.assertEqual(voided["status"], VOID)
        self.assertEqual(voided["void_reason"], "duplicate")

        with self.assertRaises(ConflictError):
            await self.service.void_invoice(invoice["id"])

        with self.assertRaises(ValidationError):
            await self.service.record_payment(invoice["id"], {"amount": "1.00"})

    async def test_paid_invoice_must_be_unpaid_before_voiding(self) -> None:
        invoice = await self.invoice()
        await self.service.issue_invoice(invoice["id"])
        payment = await self.service.record_payment(invoice["id"], {"amount": "50.00"})

        with self.assertRaises(ConflictError) as ctx:
            await self.service.void_invoice(invoice["id"])
        self.assertEqual(ctx.exception.code, "invoice_has_payments")

        await self.service.void_payment(invoice["id"], payment["id"])
        voided = await self.service.void_invoice(invoice["id"])
        self.assertEqual(voided["status"], VOID)

    async def test_only_drafts_can_be_deleted(self) -> None:
        invoice = await self.invoice()
        deleted = await self.service.delete_invoice(invoice["id"])
        self.assertTrue(deleted["deleted"])

        with self.assertRaises(NotFoundError):
            await self.service.get_invoice(invoice["id"])

        issued = await self.invoice()
        await self.service.issue_invoice(issued["id"])
        with self.assertRaises(ConflictError):
            await self.service.delete_invoice(issued["id"])

    # ------------------------------------------------------------------
    # Customers, listings and reports
    # ------------------------------------------------------------------

    async def test_archive_customer_requires_settled_invoices(self) -> None:
        customer = await self.customer()
        invoice = await self.invoice(customer["id"])
        await self.service.issue_invoice(invoice["id"])

        with self.assertRaises(ConflictError) as ctx:
            await self.service.archive_customer(customer["id"])
        self.assertEqual(ctx.exception.code, "customer_has_open_invoices")

        await self.service.void_invoice(invoice["id"])
        archived = await self.service.archive_customer(customer["id"])
        self.assertEqual(archived["status"], "archived")

        with self.assertRaises(ValidationError):
            await self.service.create_invoice(
                {"customer_id": customer["id"], "line_items": INVOICE_PAYLOAD["line_items"]}
            )

        restored = await self.service.restore_customer(customer["id"])
        self.assertEqual(restored["status"], ACTIVE)

    async def test_list_filters_and_overdue_derivation(self) -> None:
        customer = await self.customer()
        unpaid = await self.invoice(customer["id"])
        paid_invoice = await self.invoice(customer["id"])
        draft = await self.invoice(customer["id"])

        await self.service.issue_invoice(unpaid["id"])
        await self.service.issue_invoice(paid_invoice["id"])
        await self.service.record_payment(paid_invoice["id"], {"amount": "324.50"})

        self.clock.advance(40)

        overdue = await self.service.list_invoices(overdue=True)
        self.assertEqual([item["id"] for item in overdue["items"]], [unpaid["id"]])
        self.assertTrue(overdue["items"][0]["is_overdue"])
        self.assertEqual(overdue["items"][0]["days_overdue"], 26)
        self.assertEqual(overdue["items"][0]["customer"]["name"], "Acme Corp")

        drafts = await self.service.list_invoices(status=DRAFT)
        self.assertEqual([item["id"] for item in drafts["items"]], [draft["id"]])

        paid = await self.service.list_invoices(status=PAID)
        self.assertEqual([item["id"] for item in paid["items"]], [paid_invoice["id"]])

        by_customer = await self.service.list_invoices(customer_id=customer["id"], limit=2)
        self.assertEqual(by_customer["total"], 3)
        self.assertEqual(by_customer["count"], 2)

        by_number = await self.service.list_invoices(q="INV-2026")
        self.assertEqual(by_number["total"], 2)

    async def test_customer_listing_and_update(self) -> None:
        acme = await self.customer()
        await self.customer(name="Globex", email="ap@globex.test")

        updated = await self.service.update_customer(acme["id"], {"name": "Acme Inc"})
        self.assertEqual(updated["name"], "Acme Inc")

        listed = await self.service.list_customers(q="acme")
        self.assertEqual(listed["total"], 1)
        self.assertEqual(listed["items"][0]["name"], "Acme Inc")

        with self.assertRaises(ValidationError):
            await self.service.update_customer(acme["id"], {"email": "not-an-email"})

    async def test_summary_reports_outstanding_and_overdue(self) -> None:
        customer = await self.customer()
        unpaid = await self.invoice(customer["id"])
        settled = await self.invoice(customer["id"])
        issued = await self.service.issue_invoice(unpaid["id"])
        await self.service.issue_invoice(settled["id"])
        await self.service.record_payment(settled["id"], {"amount": "324.50"})
        self.clock.advance(40)

        summary = await self.service.summary()
        self.assertEqual(summary["total_invoices"], 2)
        self.assertEqual(summary["counts"][OPEN], 1)
        self.assertEqual(summary["counts"][PAID], 1)
        self.assertEqual(
            summary["by_currency"],
            [
                {
                    "currency": "USD",
                    "invoiced": "649.00",
                    "paid": "324.50",
                    "outstanding": "324.50",
                    "overdue": "324.50",
                    "draft": "0.00",
                }
            ],
        )
        self.assertIsNotNone(issued["number"])

    async def test_render_invoice_escapes_customer_input(self) -> None:
        customer = await self.customer(name="<script>alert(1)</script>")
        invoice = await self.invoice(customer["id"])
        await self.service.issue_invoice(invoice["id"])

        rendered = await self.service.render_invoice(invoice["id"])
        self.assertIn("324.50", rendered["html"])
        self.assertIn("&lt;script&gt;", rendered["html"])
        self.assertNotIn("<script>alert(1)</script>", rendered["html"])

    # ------------------------------------------------------------------
    # Validation and idempotency
    # ------------------------------------------------------------------

    async def test_invoice_idempotency(self) -> None:
        customer = await self.customer()
        payload = {"customer_id": customer["id"], **INVOICE_PAYLOAD}

        first = await self.service.create_invoice(payload, idempotency_key="invoice-once")
        second = await self.service.create_invoice(payload, idempotency_key="invoice-once")
        self.assertEqual(first["id"], second["id"])

        listed = await self.service.list_invoices()
        self.assertEqual(listed["total"], 1)

        with self.assertRaises(ConflictError):
            await self.service.create_invoice(
                {**payload, "notes": "different"}, idempotency_key="invoice-once"
            )

    async def test_validation_rules(self) -> None:
        customer = await self.customer()
        base = {"customer_id": customer["id"]}

        with self.assertRaises(ValidationError):
            await self.service.create_invoice({**base, "line_items": [{"description": "x", "quantity": 0, "unit_price": "1"}]})

        with self.assertRaises(ValidationError):
            await self.service.create_invoice(
                {**base, "issue_date": "2026-02-01", "due_date": "2026-01-01", "line_items": []}
            )

        with self.assertRaises(ValidationError):
            await self.service.create_invoice({**base, "currency": "US"})

        with self.assertRaises(ValidationError):
            await self.service.create_invoice({"line_items": []})

        with self.assertRaises(NotFoundError):
            await self.service.create_invoice({"customer_id": "cus_missing", "line_items": []})

    async def test_decimal_money_never_uses_floats(self) -> None:
        customer = await self.customer()
        invoice = await self.service.create_invoice(
            {
                "customer_id": customer["id"],
                "line_items": [
                    {"description": "Three-way split", "quantity": 1, "unit_price": "0.10"},
                    {"description": "Three-way split", "quantity": 1, "unit_price": "0.20"},
                ],
            }
        )
        # 0.10 + 0.20 is exactly 0.30, unlike binary floats.
        self.assertEqual(invoice["totals"]["subtotal"], "0.30")
        self.assertEqual(Decimal(invoice["totals"]["total"]), Decimal("0.30"))
