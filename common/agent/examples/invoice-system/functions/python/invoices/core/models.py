"""Domain models, validation and JSON presentation.

Stored documents are plain dicts with integer minor units. Presentation adds
decimal strings for humans and derived fields such as `is_overdue`.
"""

from __future__ import annotations

import re
import secrets
from datetime import date, datetime, timedelta, timezone
from typing import Any, Iterable

from .errors import ValidationError
from .money import (
    as_decimal,
    as_int,
    format_money,
    normalize_currency,
    round_bp,
    to_minor,
)

DRAFT = "draft"
OPEN = "open"
PARTIALLY_PAID = "partially_paid"
PAID = "paid"
VOID = "void"

INVOICE_STATUSES = (DRAFT, OPEN, PARTIALLY_PAID, PAID, VOID)
PAYABLE_STATUSES = (OPEN, PARTIALLY_PAID)

ACTIVE = "active"
ARCHIVED = "archived"

EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")
CURRENCY_LOCKED_STATUSES = (OPEN, PARTIALLY_PAID, PAID)


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def iso(ts: datetime) -> str:
    return ts.astimezone(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{secrets.token_hex(6)}"


def today_iso(now: datetime) -> str:
    return now.astimezone(timezone.utc).date().isoformat()


def parse_date(value: Any, *, field: str) -> str:
    if isinstance(value, datetime):
        return value.date().isoformat()
    if isinstance(value, date):
        return value.isoformat()
    if not isinstance(value, str):
        raise ValidationError(f"{field} must be an ISO date (YYYY-MM-DD)")
    try:
        return date.fromisoformat(value.strip()).isoformat()
    except ValueError as exc:
        raise ValidationError(
            f"{field} must be an ISO date (YYYY-MM-DD)", details={"value": value}
        ) from exc


def add_days(iso_date: str, days: int) -> str:
    return (date.fromisoformat(iso_date) + timedelta(days=days)).isoformat()


def _text(
    payload: dict[str, Any],
    key: str,
    *,
    required: bool = False,
    default: str | None = None,
    max_length: int = 500,
    field: str | None = None,
) -> str | None:
    name = field or key
    value = payload.get(key, default)
    if value is None:
        if required:
            raise ValidationError(f"{name} is required")
        return default
    if not isinstance(value, str):
        raise ValidationError(f"{name} must be a string")
    value = value.strip()
    if required and not value:
        raise ValidationError(f"{name} is required")
    if len(value) > max_length:
        raise ValidationError(f"{name} must be at most {max_length} characters")
    return value or None


def _metadata(payload: dict[str, Any]) -> dict[str, str]:
    value = payload.get("metadata", {})
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ValidationError("metadata must be an object")
    result: dict[str, str] = {}
    for key, item in value.items():
        if not isinstance(key, str) or len(key) > 100:
            raise ValidationError("metadata keys must be strings of at most 100 chars")
        if not isinstance(item, (str, int, float, bool)):
            raise ValidationError(f"metadata.{key} must be a scalar value")
        result[key] = str(item)
    if len(result) > 50:
        raise ValidationError("metadata is limited to 50 keys")
    return result


def _email(payload: dict[str, Any], key: str = "email") -> str | None:
    value = _text(payload, key, max_length=320)
    if value is None:
        return None
    if not EMAIL_RE.match(value):
        raise ValidationError(f"{key} must be a valid email address", details={"value": value})
    return value


def _address(payload: dict[str, Any]) -> dict[str, str] | None:
    value = payload.get("address")
    if value is None:
        return None
    if not isinstance(value, dict):
        raise ValidationError("address must be an object")
    allowed = ("line1", "line2", "city", "state", "postal_code", "country")
    address: dict[str, str] = {}
    for key in allowed:
        item = value.get(key)
        if item is None:
            continue
        if not isinstance(item, str):
            raise ValidationError(f"address.{key} must be a string")
        if len(item) > 200:
            raise ValidationError(f"address.{key} is too long")
        address[key] = item.strip()
    if not address.get("line1"):
        raise ValidationError("address.line1 is required when address is provided")
    return address


# ---------------------------------------------------------------------------
# Customers
# ---------------------------------------------------------------------------


def build_customer(payload: dict[str, Any], now: datetime, *, customer_id: str | None = None) -> dict[str, Any]:
    currency = normalize_currency(payload.get("currency", "USD"))
    terms = payload.get("payment_terms_days", 14)
    if isinstance(terms, bool) or not isinstance(terms, int):
        raise ValidationError("payment_terms_days must be an integer")
    if terms < 0 or terms > 365:
        raise ValidationError("payment_terms_days must be between 0 and 365")
    stamp = iso(now)
    return {
        "id": customer_id or new_id("cus"),
        "name": _text(payload, "name", required=True, max_length=200),
        "email": _email(payload),
        "phone": _text(payload, "phone", max_length=50),
        "tax_id": _text(payload, "tax_id", max_length=100),
        "currency": currency,
        "address": _address(payload),
        "payment_terms_days": terms,
        "status": ACTIVE,
        "metadata": _metadata(payload),
        "created_at": stamp,
        "updated_at": stamp,
    }


def apply_customer_patch(customer: dict[str, Any], payload: dict[str, Any], now: datetime) -> dict[str, Any]:
    updated = dict(customer)
    if "name" in payload:
        updated["name"] = _text(payload, "name", required=True, max_length=200)
    if "email" in payload:
        updated["email"] = _email(payload)
    if "phone" in payload:
        updated["phone"] = _text(payload, "phone", max_length=50)
    if "tax_id" in payload:
        updated["tax_id"] = _text(payload, "tax_id", max_length=100)
    if "currency" in payload:
        updated["currency"] = normalize_currency(payload["currency"])
    if "address" in payload:
        updated["address"] = _address(payload)
    if "payment_terms_days" in payload:
        terms = payload["payment_terms_days"]
        if isinstance(terms, bool) or not isinstance(terms, int) or not 0 <= terms <= 365:
            raise ValidationError("payment_terms_days must be an integer between 0 and 365")
        updated["payment_terms_days"] = terms
    if "metadata" in payload:
        updated["metadata"] = _metadata(payload)
    updated["updated_at"] = iso(now)
    return updated


# ---------------------------------------------------------------------------
# Line items and totals
# ---------------------------------------------------------------------------


def build_line_items(raw_items: Any, currency: str, *, required_min: int = 0) -> list[dict[str, Any]]:
    if raw_items is None:
        raw_items = []
    if not isinstance(raw_items, list):
        raise ValidationError("line_items must be an array")
    if len(raw_items) > 200:
        raise ValidationError("line_items is limited to 200 entries")
    if len(raw_items) < required_min:
        raise ValidationError("at least one line item is required")

    items: list[dict[str, Any]] = []
    for index, raw in enumerate(raw_items):
        field = f"line_items[{index}]"
        if not isinstance(raw, dict):
            raise ValidationError(f"{field} must be an object")
        description = _text(raw, "description", required=True, max_length=500, field=f"{field}.description")
        quantity = as_decimal(raw.get("quantity", 1), field=f"{field}.quantity")
        if quantity <= 0:
            raise ValidationError(f"{field}.quantity must be greater than zero")
        if quantity > 1_000_000:
            raise ValidationError(f"{field}.quantity is unreasonably large")
        unit_price = as_decimal(raw.get("unit_price", 0), field=f"{field}.unit_price")
        if unit_price < 0:
            raise ValidationError(f"{field}.unit_price must not be negative")
        tax_bp = as_int(raw.get("tax_rate_bp", 0), field=f"{field}.tax_rate_bp", minimum=0, maximum=10_000)
        discount_bp = as_int(raw.get("discount_bp", 0), field=f"{field}.discount_bp", minimum=0, maximum=10_000)

        amount_minor = to_minor(quantity * unit_price, currency)
        discount_minor = round_bp(amount_minor, discount_bp)
        net_minor = amount_minor - discount_minor
        tax_minor = round_bp(net_minor, tax_bp)
        items.append(
            {
                "id": _text(raw, "id", max_length=64) or new_id("li"),
                "description": description,
                "quantity": f"{quantity.normalize():f}",
                "unit_price": f"{unit_price:.{_exponent(currency)}f}",
                "tax_rate_bp": tax_bp,
                "discount_bp": discount_bp,
                "amount_minor": amount_minor,
                "discount_minor": discount_minor,
                "tax_minor": tax_minor,
                "total_minor": net_minor + tax_minor,
            }
        )
    return items


def _exponent(currency: str) -> int:
    from .money import exponent_for

    return exponent_for(currency)


def totals_for(items: Iterable[dict[str, Any]]) -> dict[str, int]:
    subtotal = sum(item["amount_minor"] for item in items)
    discount = sum(item["discount_minor"] for item in items)
    tax = sum(item["tax_minor"] for item in items)
    return {
        "subtotal_minor": subtotal,
        "discount_minor": discount,
        "tax_minor": tax,
        "total_minor": subtotal - discount + tax,
    }


def build_invoice(
    payload: dict[str, Any],
    customer: dict[str, Any],
    now: datetime,
) -> dict[str, Any]:
    currency = normalize_currency(payload.get("currency", customer["currency"]))
    issue_date = parse_date(payload.get("issue_date", today_iso(now)), field="issue_date")
    due_date = parse_date(
        payload.get("due_date", add_days(issue_date, customer["payment_terms_days"])),
        field="due_date",
    )
    if due_date < issue_date:
        raise ValidationError("due_date must not be before issue_date")

    items = build_line_items(payload.get("line_items"), currency)
    totals = totals_for(items)
    stamp = iso(now)
    return {
        "id": new_id("inv"),
        "number": None,
        "status": DRAFT,
        "customer_id": customer["id"],
        "customer": None,
        "currency": currency,
        "issue_date": issue_date,
        "due_date": due_date,
        "notes": _text(payload, "notes", max_length=5000),
        "terms": _text(payload, "terms", max_length=5000),
        "line_items": items,
        "subtotal_minor": totals["subtotal_minor"],
        "discount_minor": totals["discount_minor"],
        "tax_minor": totals["tax_minor"],
        "total_minor": totals["total_minor"],
        "amount_paid_minor": 0,
        "balance_minor": totals["total_minor"],
        "metadata": _metadata(payload),
        "created_at": stamp,
        "updated_at": stamp,
        "issued_at": None,
        "sent_at": None,
        "voided_at": None,
        "void_reason": None,
    }


def apply_invoice_patch(invoice: dict[str, Any], payload: dict[str, Any], now: datetime) -> dict[str, Any]:
    if invoice["status"] != DRAFT:
        raise ValidationError("only draft invoices can be edited")
    updated = dict(invoice)

    currency = updated["currency"]
    if "currency" in payload:
        requested = normalize_currency(payload["currency"])
        if requested != currency:
            raise ValidationError(
                "currency cannot be changed on an existing invoice; create a new draft"
            )
    if "issue_date" in payload:
        updated["issue_date"] = parse_date(payload["issue_date"], field="issue_date")
    if "due_date" in payload:
        updated["due_date"] = parse_date(payload["due_date"], field="due_date")
    elif "issue_date" in payload and not payload.get("due_date"):
        terms = date.fromisoformat(updated["due_date"]) - date.fromisoformat(invoice["issue_date"])
        updated["due_date"] = add_days(updated["issue_date"], max(terms.days, 0))
    if updated["due_date"] < updated["issue_date"]:
        raise ValidationError("due_date must not be before issue_date")

    if "notes" in payload:
        updated["notes"] = _text(payload, "notes", max_length=5000)
    if "terms" in payload:
        updated["terms"] = _text(payload, "terms", max_length=5000)
    if "metadata" in payload:
        updated["metadata"] = _metadata(payload)
    if "line_items" in payload or currency != updated["currency"]:
        if "line_items" in payload:
            updated["line_items"] = build_line_items(payload["line_items"], currency)
        else:
            updated["line_items"] = build_line_items(
                _items_for_rebuild(updated["line_items"]), currency
            )

    updated["currency"] = currency
    totals = totals_for(updated["line_items"])
    updated.update(totals)
    updated["balance_minor"] = updated["total_minor"] - updated["amount_paid_minor"]
    updated["updated_at"] = iso(now)
    return updated


def _items_for_rebuild(items: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Turn stored line items back into the shape build_line_items accepts."""
    return [
        {
            "id": item["id"],
            "description": item["description"],
            "quantity": item["quantity"],
            "unit_price": item["unit_price"],
            "tax_rate_bp": item["tax_rate_bp"],
            "discount_bp": item["discount_bp"],
        }
        for item in items
    ]


# ---------------------------------------------------------------------------
# Presentation
# ---------------------------------------------------------------------------


def present_customer(customer: dict[str, Any]) -> dict[str, Any]:
    return dict(customer)


def present_line_item(item: dict[str, Any], currency: str) -> dict[str, Any]:
    return {
        "id": item["id"],
        "description": item["description"],
        "quantity": item["quantity"],
        "unit_price": item["unit_price"],
        "tax_rate_bp": item["tax_rate_bp"],
        "discount_bp": item["discount_bp"],
        "amount": format_money(item["amount_minor"], currency),
        "discount": format_money(item["discount_minor"], currency),
        "tax": format_money(item["tax_minor"], currency),
        "total": format_money(item["total_minor"], currency),
    }


def present_invoice(invoice: dict[str, Any], today: date) -> dict[str, Any]:
    currency = invoice["currency"]
    due = date.fromisoformat(invoice["due_date"])
    overdue = invoice["status"] in PAYABLE_STATUSES and invoice["balance_minor"] > 0 and due < today
    return {
        "id": invoice["id"],
        "number": invoice["number"],
        "status": invoice["status"],
        "customer_id": invoice["customer_id"],
        "customer": invoice["customer"],
        "currency": currency,
        "issue_date": invoice["issue_date"],
        "due_date": invoice["due_date"],
        "notes": invoice["notes"],
        "terms": invoice["terms"],
        "line_items": [present_line_item(item, currency) for item in invoice["line_items"]],
        "totals": {
            "subtotal": format_money(invoice["subtotal_minor"], currency),
            "discount": format_money(invoice["discount_minor"], currency),
            "tax": format_money(invoice["tax_minor"], currency),
            "total": format_money(invoice["total_minor"], currency),
            "amount_paid": format_money(invoice["amount_paid_minor"], currency),
            "balance_due": format_money(invoice["balance_minor"], currency),
        },
        "metadata": invoice["metadata"],
        "created_at": invoice["created_at"],
        "updated_at": invoice["updated_at"],
        "issued_at": invoice["issued_at"],
        "sent_at": invoice["sent_at"],
        "voided_at": invoice["voided_at"],
        "void_reason": invoice.get("void_reason"),
        "is_overdue": overdue,
        "days_overdue": max((today - due).days, 0) if overdue else 0,
    }


def present_payment(payment: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": payment["id"],
        "invoice_id": payment["invoice_id"],
        "amount": format_money(payment["amount_minor"], payment["currency"]),
        "currency": payment["currency"],
        "method": payment["method"],
        "reference": payment["reference"],
        "paid_at": payment["paid_at"],
        "created_at": payment["created_at"],
        "voided_at": payment["voided_at"],
        "metadata": payment["metadata"],
    }
