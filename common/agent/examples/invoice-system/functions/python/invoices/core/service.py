"""Invoice application service.

Everything the HTTP routes and async ops can do lives here, so the transport is
replaceable and the whole domain is testable against `MemoryStore`.
"""

from __future__ import annotations

import hashlib
import html
import json
import logging
from datetime import date, datetime
from typing import Any, Awaitable, Callable

from . import models as m
from .errors import (
    ConflictError,
    KeyExistsError,
    NotFoundError,
    RevisionMismatchError,
    ValidationError,
)
from .money import format_money, normalize_currency, parse_money
from .store import (
    CUSTOMERS,
    INVOICES,
    META,
    PAYMENTS,
    Store,
    encode_key_part,
)

logger = logging.getLogger(__name__)

Publisher = Callable[[str, dict[str, Any]], Awaitable[None]]


def _dump(doc: dict[str, Any]) -> bytes:
    return json.dumps(doc, separators=(",", ":"), sort_keys=True).encode("utf-8")


def _fingerprint(payload: Any) -> str:
    blob = json.dumps(payload, sort_keys=True, separators=(",", ":"), default=str)
    return hashlib.sha256(blob.encode("utf-8")).hexdigest()


class InvoiceService:
    def __init__(
        self,
        store: Store,
        *,
        clock: Callable[[], datetime] = m.utcnow,
        publisher: Publisher | None = None,
    ) -> None:
        self.store = store
        self.clock = clock
        self.publisher = publisher

    # ------------------------------------------------------------------
    # Storage helpers
    # ------------------------------------------------------------------

    async def _read(self, bucket: str, key: str) -> tuple[dict[str, Any], int] | None:
        value = await self.store.get(bucket, key)
        if value is None:
            return None
        try:
            return json.loads(value.data.decode("utf-8")), value.revision
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ConflictError(
                f"stored document {bucket}/{key} is corrupt", code="corrupt_document"
            ) from exc

    async def _require(self, bucket: str, key: str, label: str) -> tuple[dict[str, Any], int]:
        found = await self._read(bucket, key)
        if found is None:
            raise NotFoundError(f"{label} not found", details={"id": key.split(".")[-1]})
        return found

    async def _write(self, bucket: str, key: str, doc: dict[str, Any]) -> int:
        value = await self.store.put(bucket, key, _dump(doc))
        return value.revision

    async def _insert(self, bucket: str, key: str, doc: dict[str, Any]) -> int:
        try:
            value = await self.store.create(bucket, key, _dump(doc))
        except KeyExistsError as exc:
            raise ConflictError(f"{bucket}/{key} already exists", code="already_exists") from exc
        return value.revision

    async def _list(self, bucket: str, prefix: str) -> list[dict[str, Any]]:
        docs: list[dict[str, Any]] = []
        for key in await self.store.keys(bucket, prefix):
            value = await self.store.get(bucket, key)
            if value is None:
                continue
            try:
                docs.append(json.loads(value.data.decode("utf-8")))
            except (UnicodeDecodeError, json.JSONDecodeError):
                logger.warning("skipping corrupt document bucket=%s key=%s", bucket, key)
        return docs

    async def _emit(self, event: str, payload: dict[str, Any]) -> None:
        if self.publisher is None:
            return
        try:
            await self.publisher(event, payload)
        except Exception:
            logger.exception("failed to publish event %s", event)

    async def _idempotent(
        self,
        scope: str,
        key: str | None,
        payload: Any,
        create: Callable[[], Awaitable[dict[str, Any]]],
        load: Callable[[str], Awaitable[dict[str, Any] | None]],
    ) -> dict[str, Any]:
        if not key:
            return await create()

        meta_key = f"idem.{scope}.{encode_key_part(key)}"
        fingerprint = _fingerprint(payload)
        try:
            await self.store.create(
                META,
                meta_key,
                _dump({"state": "pending", "fingerprint": fingerprint, "id": None}),
            )
        except KeyExistsError:
            existing = await self._read(META, meta_key)
            if existing is None:
                raise ConflictError(
                    "idempotent request state disappeared; retry", code="idempotency_state_lost"
                )
            state, _revision = existing
            if state.get("fingerprint") != fingerprint:
                raise ConflictError(
                    "Idempotency-Key was already used with a different request",
                    code="idempotency_key_reused",
                )
            if not state.get("id"):
                raise ConflictError(
                    "a request with this Idempotency-Key is still in progress",
                    code="idempotency_in_progress",
                )
            found = await load(state["id"])
            if found is None:
                raise ConflictError(
                    "the original idempotent result no longer exists",
                    code="idempotency_result_missing",
                )
            return found

        try:
            doc = await create()
        except Exception:
            await self.store.delete(META, meta_key)
            raise
        await self._write(
            META,
            meta_key,
            {"state": "done", "fingerprint": fingerprint, "id": doc["id"]},
        )
        return doc

    # ------------------------------------------------------------------
    # Customers
    # ------------------------------------------------------------------

    async def create_customer(
        self, payload: dict[str, Any], idempotency_key: str | None = None
    ) -> dict[str, Any]:
        if not isinstance(payload, dict):
            raise ValidationError("customer payload must be an object")

        async def create() -> dict[str, Any]:
            customer = m.build_customer(payload, self.clock())
            await self._insert(CUSTOMERS, f"cust.{customer['id']}", customer)
            await self._emit("customer.created", customer)
            return customer

        doc = await self._idempotent(
            "customers",
            idempotency_key,
            payload,
            create,
            lambda id: self._read_doc(CUSTOMERS, f"cust.{id}"),
        )
        return m.present_customer(doc)

    async def get_customer(self, customer_id: str) -> dict[str, Any]:
        doc, _ = await self._require(CUSTOMERS, f"cust.{customer_id}", "customer")
        return m.present_customer(doc)

    async def list_customers(
        self,
        *,
        status: str | None = None,
        q: str | None = None,
        limit: int = 50,
        offset: int = 0,
    ) -> dict[str, Any]:
        if status not in (None, "all", m.ACTIVE, m.ARCHIVED):
            raise ValidationError("status must be active, archived or all")
        docs = await self._list(CUSTOMERS, "cust.")
        if status not in (None, "all"):
            docs = [doc for doc in docs if doc["status"] == status]
        if q:
            needle = q.lower()
            docs = [
                doc
                for doc in docs
                if needle in doc["name"].lower() or needle in (doc.get("email") or "").lower()
            ]
        docs.sort(key=lambda doc: doc["created_at"], reverse=True)
        total = len(docs)
        window = docs[offset : offset + limit]
        return {
            "items": [m.present_customer(doc) for doc in window],
            "count": len(window),
            "total": total,
            "limit": limit,
            "offset": offset,
        }

    async def update_customer(self, customer_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        doc, revision = await self._require(CUSTOMERS, f"cust.{customer_id}", "customer")
        updated = m.apply_customer_patch(doc, payload, self.clock())
        await self.store.update(CUSTOMERS, f"cust.{customer_id}", _dump(updated), revision)
        await self._emit("customer.updated", updated)
        return m.present_customer(updated)

    async def archive_customer(self, customer_id: str) -> dict[str, Any]:
        doc, _ = await self._require(CUSTOMERS, f"cust.{customer_id}", "customer")
        if doc["status"] == m.ARCHIVED:
            return m.present_customer(doc)
        open_invoices = [
            invoice
            for invoice in await self._list(INVOICES, "inv.")
            if invoice["customer_id"] == customer_id and invoice["status"] in m.PAYABLE_STATUSES
        ]
        if open_invoices:
            raise ConflictError(
                "customer has unpaid invoices; settle or void them first",
                code="customer_has_open_invoices",
                details={"invoice_ids": [inv["id"] for inv in open_invoices]},
            )
        doc["status"] = m.ARCHIVED
        doc["updated_at"] = m.iso(self.clock())
        await self._write(CUSTOMERS, f"cust.{customer_id}", doc)
        await self._emit("customer.archived", doc)
        return m.present_customer(doc)

    async def restore_customer(self, customer_id: str) -> dict[str, Any]:
        doc, _ = await self._require(CUSTOMERS, f"cust.{customer_id}", "customer")
        doc["status"] = m.ACTIVE
        doc["updated_at"] = m.iso(self.clock())
        await self._write(CUSTOMERS, f"cust.{customer_id}", doc)
        await self._emit("customer.restored", doc)
        return m.present_customer(doc)

    # ------------------------------------------------------------------
    # Invoices
    # ------------------------------------------------------------------

    async def create_invoice(
        self, payload: dict[str, Any], idempotency_key: str | None = None
    ) -> dict[str, Any]:
        if not isinstance(payload, dict):
            raise ValidationError("invoice payload must be an object")
        customer_id = payload.get("customer_id")
        if not isinstance(customer_id, str) or not customer_id.strip():
            raise ValidationError("customer_id is required")

        async def create() -> dict[str, Any]:
            customer, _ = await self._require(
                CUSTOMERS, f"cust.{customer_id}", "customer"
            )
            if customer["status"] != m.ACTIVE:
                raise ValidationError("cannot invoice an archived customer")
            invoice = m.build_invoice(payload, customer, self.clock())
            await self._insert(INVOICES, f"inv.{invoice['id']}", invoice)
            await self._emit("invoice.created", self._present(invoice))
            return invoice

        doc = await self._idempotent(
            "invoices",
            idempotency_key,
            payload,
            create,
            lambda id: self._read_doc(INVOICES, f"inv.{id}"),
        )
        return self._present(doc)

    async def get_invoice(self, invoice_id: str) -> dict[str, Any]:
        doc, _ = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        return self._present(doc)

    async def list_invoices(
        self,
        *,
        status: str | None = None,
        customer_id: str | None = None,
        q: str | None = None,
        overdue: bool | None = None,
        limit: int = 50,
        offset: int = 0,
    ) -> dict[str, Any]:
        if status not in (None, "all", *m.INVOICE_STATUSES):
            raise ValidationError(
                "status must be one of " + ", ".join(("all", *m.INVOICE_STATUSES))
            )
        docs = await self._list(INVOICES, "inv.")
        if status not in (None, "all"):
            docs = [doc for doc in docs if doc["status"] == status]
        if customer_id:
            docs = [doc for doc in docs if doc["customer_id"] == customer_id]
        if q:
            needle = q.lower()
            docs = [
                doc
                for doc in docs
                if needle in doc["id"].lower()
                or needle in (doc.get("number") or "").lower()
                or needle in ((doc.get("customer") or {}).get("name") or "").lower()
            ]

        today = self.clock().date()
        presented = [m.present_invoice(doc, today) for doc in docs]
        if overdue is not None:
            presented = [doc for doc in presented if doc["is_overdue"] is overdue]
        presented.sort(key=lambda doc: (doc["created_at"], doc["id"]), reverse=True)
        total = len(presented)
        window = presented[offset : offset + limit]
        return {
            "items": window,
            "count": len(window),
            "total": total,
            "limit": limit,
            "offset": offset,
        }

    async def update_invoice(self, invoice_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        doc, revision = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        updated = m.apply_invoice_patch(doc, payload, self.clock())
        await self.store.update(INVOICES, f"inv.{invoice_id}", _dump(updated), revision)
        await self._emit("invoice.updated", self._present(updated))
        return self._present(updated)

    async def issue_invoice(
        self, invoice_id: str, payload: dict[str, Any] | None = None
    ) -> dict[str, Any]:
        payload = payload or {}
        current, _ = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        if current["status"] != m.DRAFT:
            raise ValidationError("only draft invoices can be issued")
        if not current["line_items"]:
            raise ValidationError(
                "an invoice needs at least one line item before it is issued"
            )
        issue_date = m.parse_date(
            payload.get("issue_date", current["issue_date"]), field="issue_date"
        )
        due_date = m.parse_date(
            payload.get("due_date", current["due_date"]), field="due_date"
        )
        if due_date < issue_date:
            raise ValidationError("due_date must not be before issue_date")

        customer, _ = await self._require(
            CUSTOMERS, f"cust.{current['customer_id']}", "customer"
        )
        if customer["status"] != m.ACTIVE:
            raise ValidationError("cannot issue an invoice for an archived customer")

        number = await self._next_number(issue_date[:4])

        def mutate(doc: dict[str, Any]) -> dict[str, Any]:
            if doc["status"] != m.DRAFT:
                raise ValidationError("only draft invoices can be issued")
            doc["issue_date"] = issue_date
            doc["due_date"] = due_date
            doc["number"] = number
            doc["customer"] = m.present_customer(customer)
            doc["status"] = m.OPEN
            doc["issued_at"] = m.iso(self.clock())
            return doc

        doc = await self._mutate_invoice(invoice_id, mutate)
        await self._emit("invoice.issued", self._present(doc))
        return self._present(doc)

    async def void_invoice(
        self, invoice_id: str, payload: dict[str, Any] | None = None
    ) -> dict[str, Any]:
        payload = payload or {}
        reason = payload.get("reason")
        if reason is not None and (not isinstance(reason, str) or len(reason) > 500):
            raise ValidationError("reason must be a string of at most 500 characters")

        def mutate(doc: dict[str, Any]) -> dict[str, Any]:
            if doc["status"] == m.VOID:
                raise ConflictError("invoice is already void", code="already_void")
            if doc["status"] == m.DRAFT:
                raise ValidationError("draft invoices should be deleted, not voided")
            if doc["amount_paid_minor"] > 0:
                raise ConflictError(
                    "invoice has recorded payments; void the payments first",
                    code="invoice_has_payments",
                )
            doc["status"] = m.VOID
            doc["voided_at"] = m.iso(self.clock())
            doc["void_reason"] = reason
            return doc

        doc = await self._mutate_invoice(invoice_id, mutate)
        await self._emit("invoice.voided", self._present(doc))
        return self._present(doc)

    async def send_invoice(self, invoice_id: str) -> dict[str, Any]:
        def mutate(doc: dict[str, Any]) -> dict[str, Any]:
            if doc["status"] not in (*m.PAYABLE_STATUSES, m.PAID):
                raise ValidationError("only issued invoices can be sent")
            doc["sent_at"] = m.iso(self.clock())
            return doc

        doc = await self._mutate_invoice(invoice_id, mutate)
        await self._emit("invoice.sent", self._present(doc))
        return self._present(doc)

    async def delete_invoice(self, invoice_id: str) -> dict[str, Any]:
        doc, _ = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        if doc["status"] != m.DRAFT:
            raise ConflictError(
                "only draft invoices can be deleted; void issued invoices instead",
                code="invoice_not_draft",
            )
        await self.store.delete(INVOICES, f"inv.{invoice_id}")
        await self._emit("invoice.deleted", {"id": invoice_id})
        return {"id": invoice_id, "deleted": True}

    # ------------------------------------------------------------------
    # Payments
    # ------------------------------------------------------------------

    async def record_payment(
        self,
        invoice_id: str,
        payload: dict[str, Any],
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        if not isinstance(payload, dict):
            raise ValidationError("payment payload must be an object")

        async def create() -> dict[str, Any]:
            invoice, _ = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
            if invoice["status"] not in m.PAYABLE_STATUSES:
                raise ValidationError(
                    f"invoice in status '{invoice['status']}' cannot accept payments"
                )
            currency = invoice["currency"]
            if "currency" in payload:
                supplied = normalize_currency(payload["currency"])
                if supplied != currency:
                    raise ValidationError(
                        "payment currency must match the invoice currency",
                        details={"expected": currency, "received": supplied},
                    )
            amount_minor = parse_money(payload.get("amount"), currency, field="amount")
            if amount_minor <= 0:
                raise ValidationError("amount must be greater than zero")
            if amount_minor > invoice["balance_minor"]:
                raise ValidationError(
                    "payment exceeds the outstanding balance",
                    details={
                        "balance_due": format_money(invoice["balance_minor"], currency),
                        "amount": format_money(amount_minor, currency),
                    },
                )
            method = payload.get("method", "bank_transfer")
            if not isinstance(method, str) or not 1 <= len(method) <= 50:
                raise ValidationError("method must be a string of 1-50 characters")
            reference = payload.get("reference")
            if reference is not None and (
                not isinstance(reference, str) or len(reference) > 200
            ):
                raise ValidationError("reference must be a string of at most 200 characters")
            paid_at = m.parse_date(
                payload.get("paid_at", m.today_iso(self.clock())), field="paid_at"
            )
            metadata = payload.get("metadata") or {}
            if not isinstance(metadata, dict):
                raise ValidationError("metadata must be an object")

            payment = {
                "id": m.new_id("pay"),
                "invoice_id": invoice_id,
                "amount_minor": amount_minor,
                "currency": currency,
                "method": method,
                "reference": reference,
                "paid_at": paid_at,
                "metadata": {str(k): str(v) for k, v in metadata.items()},
                "created_at": m.iso(self.clock()),
                "voided_at": None,
            }
            await self._insert(
                PAYMENTS, f"pay.{invoice_id}.{payment['id']}", payment
            )
            try:
                updated = await self._mutate_invoice(
                    invoice_id,
                    lambda doc: self._apply_payment(doc, amount_minor, self.clock()),
                )
            except Exception:
                await self.store.delete(PAYMENTS, f"pay.{invoice_id}.{payment['id']}")
                raise
            await self._emit("payment.recorded", m.present_payment(payment))
            if updated["status"] == m.PAID:
                await self._emit("invoice.paid", self._present(updated))
            return payment

        doc = await self._idempotent(
            "payments",
            idempotency_key,
            {"invoice_id": invoice_id, **payload},
            create,
            lambda id: self._read_doc(PAYMENTS, f"pay.{invoice_id}.{id}"),
        )
        return m.present_payment(doc)

    async def list_payments(self, invoice_id: str) -> dict[str, Any]:
        await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        docs = await self._list(PAYMENTS, f"pay.{invoice_id}.")
        docs.sort(key=lambda doc: (doc["created_at"], doc["id"]), reverse=True)
        return {
            "items": [m.present_payment(doc) for doc in docs],
            "count": len(docs),
            "invoice_id": invoice_id,
        }

    async def void_payment(
        self, invoice_id: str, payment_id: str, payload: dict[str, Any] | None = None
    ) -> dict[str, Any]:
        payload = payload or {}
        payment, revision = await self._require(
            PAYMENTS, f"pay.{invoice_id}.{payment_id}", "payment"
        )
        if payment["voided_at"]:
            raise ConflictError("payment is already void", code="payment_already_void")
        payment["voided_at"] = m.iso(self.clock())
        reason = payload.get("reason")
        if reason is not None:
            if not isinstance(reason, str) or len(reason) > 500:
                raise ValidationError("reason must be a string of at most 500 characters")
            payment["void_reason"] = reason
        await self.store.update(
            PAYMENTS, f"pay.{invoice_id}.{payment_id}", _dump(payment), revision
        )
        updated = await self._mutate_invoice(
            invoice_id,
            lambda doc: self._apply_payment(doc, -payment["amount_minor"], self.clock()),
        )
        await self._emit("payment.voided", m.present_payment(payment))
        return {"payment": m.present_payment(payment), "invoice": self._present(updated)}

    # ------------------------------------------------------------------
    # Reporting and rendering
    # ------------------------------------------------------------------

    async def summary(self) -> dict[str, Any]:
        invoices = await self._list(INVOICES, "inv.")
        today = self.clock().date()
        by_currency: dict[str, dict[str, Any]] = {}
        counts = {status: 0 for status in m.INVOICE_STATUSES}
        for invoice in invoices:
            currency = invoice["currency"]
            bucket = by_currency.setdefault(
                currency,
                {
                    "currency": currency,
                    "invoiced": 0,
                    "paid": 0,
                    "outstanding": 0,
                    "overdue": 0,
                    "draft": 0,
                },
            )
            counts[invoice["status"]] = counts.get(invoice["status"], 0) + 1
            if invoice["status"] == m.DRAFT:
                bucket["draft"] += invoice["total_minor"]
                continue
            if invoice["status"] == m.VOID:
                continue
            bucket["invoiced"] += invoice["total_minor"]
            bucket["paid"] += invoice["amount_paid_minor"]
            if invoice["balance_minor"] > 0:
                bucket["outstanding"] += invoice["balance_minor"]
                if date.fromisoformat(invoice["due_date"]) < today:
                    bucket["overdue"] += invoice["balance_minor"]

        rendered = []
        for currency, amounts in sorted(by_currency.items()):
            rendered.append(
                {
                    "currency": currency,
                    "invoiced": format_money(amounts["invoiced"], currency),
                    "paid": format_money(amounts["paid"], currency),
                    "outstanding": format_money(amounts["outstanding"], currency),
                    "overdue": format_money(amounts["overdue"], currency),
                    "draft": format_money(amounts["draft"], currency),
                }
            )
        return {
            "as_of": m.iso(self.clock()),
            "counts": counts,
            "total_invoices": len(invoices),
            "by_currency": rendered,
        }

    async def render_invoice(self, invoice_id: str) -> dict[str, Any]:
        doc, _ = await self._require(INVOICES, f"inv.{invoice_id}", "invoice")
        presented = self._present(doc)
        customer = doc.get("customer")
        if not customer:
            customer = m.present_customer(
                (await self._require(CUSTOMERS, f"cust.{doc['customer_id']}", "customer"))[0]
            )
        return {"invoice_id": invoice_id, "html": _invoice_html(presented, customer)}

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    async def _mutate_invoice(
        self,
        invoice_id: str,
        mutate: Callable[[dict[str, Any]], dict[str, Any]],
    ) -> dict[str, Any]:
        key = f"inv.{invoice_id}"
        for _attempt in range(8):
            doc, revision = await self._require(INVOICES, key, "invoice")
            updated = mutate(dict(doc))
            updated["updated_at"] = m.iso(self.clock())
            try:
                await self.store.update(INVOICES, key, _dump(updated), revision)
                return updated
            except RevisionMismatchError:
                continue
        raise ConflictError(
            "invoice was modified concurrently; retry the request",
            code="write_conflict",
        )

    def _apply_payment(
        self, invoice: dict[str, Any], delta_minor: int, now: datetime
    ) -> dict[str, Any]:
        paid = invoice["amount_paid_minor"] + delta_minor
        if paid < 0:
            paid = 0
        invoice["amount_paid_minor"] = paid
        invoice["balance_minor"] = invoice["total_minor"] - paid
        invoice["updated_at"] = m.iso(now)
        if invoice["status"] == m.VOID:
            return invoice
        if invoice["balance_minor"] <= 0:
            invoice["status"] = m.PAID
        elif paid > 0:
            invoice["status"] = m.PARTIALLY_PAID
        else:
            invoice["status"] = m.OPEN
        return invoice

    async def _next_number(self, year: str) -> str:
        key = f"seq.invoice.{year}"
        for _attempt in range(25):
            current = await self.store.get(META, key)
            if current is None:
                try:
                    await self.store.create(META, key, b"1")
                    sequence = 1
                    break
                except KeyExistsError:
                    continue
            try:
                sequence = int(current.data.decode("ascii")) + 1
            except ValueError as exc:
                raise ConflictError(
                    f"invoice sequence for {year} is corrupt", code="corrupt_sequence"
                ) from exc
            try:
                await self.store.update(META, key, str(sequence).encode("ascii"), current.revision)
                break
            except RevisionMismatchError:
                continue
        else:
            raise ConflictError(
                "could not allocate an invoice number; retry", code="numbering_contention"
            )
        return f"INV-{year}-{sequence:04d}"

    def _present(self, invoice: dict[str, Any]) -> dict[str, Any]:
        return m.present_invoice(invoice, self.clock().date())

    async def _read_doc(self, bucket: str, key: str) -> dict[str, Any] | None:
        found = await self._read(bucket, key)
        return found[0] if found is not None else None


def _invoice_html(invoice: dict[str, Any], customer: dict[str, Any]) -> str:
    esc = html.escape

    def address_lines(value: dict[str, Any] | None) -> str:
        if not value:
            return ""
        parts = [
            value.get("line1"),
            value.get("line2"),
            " ".join(
                part
                for part in (value.get("city"), value.get("state"), value.get("postal_code"))
                if part
            ),
            value.get("country"),
        ]
        return "<br>".join(esc(part) for part in parts if part)

    rows = "".join(
        f"<tr><td>{esc(item['description'])}</td><td class='num'>{esc(item['quantity'])}</td>"
        f"<td class='num'>{esc(item['unit_price'])}</td><td class='num'>{esc(item['tax'])}</td>"
        f"<td class='num'>{esc(item['total'])}</td></tr>"
        for item in invoice["line_items"]
    )
    totals = invoice["totals"]
    currency = invoice["currency"]
    notes = f"<p class='notes'>{esc(invoice['notes'])}</p>" if invoice["notes"] else ""
    status = esc(invoice["status"].replace("_", " ").title())
    return f"""<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>Invoice {esc(invoice['number'] or invoice['id'])}</title>
<style>
body {{ font-family: -apple-system, Segoe UI, sans-serif; margin: 0; padding: 40px; color: #1c1c1e; background: #fff; }}
h1 {{ font-size: 28px; margin: 0 0 4px; }}
.muted {{ color: #6b7280; }}
.header {{ display: flex; justify-content: space-between; gap: 24px; margin-bottom: 32px; }}
table {{ width: 100%; border-collapse: collapse; margin-top: 12px; }}
th, td {{ padding: 10px 12px; border-bottom: 1px solid #e5e7eb; text-align: left; }}
th {{ font-size: 12px; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }}
.num {{ text-align: right; font-variant-numeric: tabular-nums; }}
.totals {{ margin-top: 20px; margin-left: auto; width: 320px; }}
.totals div {{ display: flex; justify-content: space-between; padding: 6px 0; }}
.totals .grand {{ border-top: 2px solid #111827; font-weight: 700; font-size: 18px; margin-top: 6px; padding-top: 10px; }}
.badge {{ display: inline-block; padding: 3px 10px; border-radius: 999px; background: #f3f4f6; font-size: 12px; }}
.notes {{ margin-top: 28px; white-space: pre-wrap; color: #374151; }}
</style></head>
<body>
<div class="header">
  <div>
    <h1>Invoice {esc(invoice['number'] or '(draft)')}</h1>
    <div class="muted">Issued {esc(invoice['issue_date'])} &middot; Due {esc(invoice['due_date'])}</div>
  </div>
  <div><span class="badge">{status}</span></div>
</div>
<div class="header">
  <div><strong>Bill to</strong><br>{esc(customer['name'])}<br>{address_lines(customer.get('address'))}
  <br>{esc(customer.get('email') or '')}</div>
  <div class="muted">Currency: {esc(currency)}</div>
</div>
<table><thead><tr><th>Description</th><th class="num">Qty</th><th class="num">Unit price</th><th class="num">Tax</th><th class="num">Total</th></tr></thead>
<tbody>{rows or '<tr><td colspan="5" class="muted">No line items</td></tr>'}</tbody></table>
<div class="totals">
  <div><span>Subtotal</span><span>{esc(totals['subtotal'])}</span></div>
  <div><span>Discount</span><span>-{esc(totals['discount'])}</span></div>
  <div><span>Tax</span><span>{esc(totals['tax'])}</span></div>
  <div class="grand"><span>Total</span><span>{esc(totals['total'])} {esc(currency)}</span></div>
  <div><span>Paid</span><span>{esc(totals['amount_paid'])}</span></div>
  <div><span>Balance due</span><span>{esc(totals['balance_due'])}</span></div>
</div>
{notes}
</body></html>"""
