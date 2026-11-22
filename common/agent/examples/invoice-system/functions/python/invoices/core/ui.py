"""Server-rendered invoice UI.

The UI is part of the same deployable function as the JSON API. Templates
live in `templates/` and assets in `static/`, both copied with the package by
the runtime. Routes return a `Page` (HTML/CSS/JS) or a redirect; the JSON routes
in `core/http.py` keep their existing envelope.
"""

from __future__ import annotations

import logging
import os
from datetime import date, timedelta
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from functools import wraps
from pathlib import Path
from typing import Any, Awaitable, Callable
from urllib.parse import parse_qs, urlencode

from . import models as m
from .errors import ApiError, ValidationError
from .page import Page, html, redirect
from .service import InvoiceService

logger = logging.getLogger(__name__)

PACKAGE_DIR = Path(__file__).resolve().parents[1]
TEMPLATES_DIR = PACKAGE_DIR / "templates"
STATIC_DIR = PACKAGE_DIR / "static"

try:  # The Python runtime ships jinja2; local dev installs it per the README.
    from jinja2 import Environment, FileSystemLoader, select_autoescape
except ImportError:  # pragma: no cover - exercised only without jinja2
    Environment = None  # type: ignore[assignment]

Handler = Callable[[Any, Any], Awaitable[Page]]

_CONTENT_TYPES = {
    ".css": "text/css; charset=utf-8",
    ".js": "application/javascript; charset=utf-8",
    ".svg": "image/svg+xml",
    ".png": "image/png",
    ".ico": "image/x-icon",
    ".woff2": "font/woff2",
    ".html": "text/html; charset=utf-8",
}


class Form:
    """Small wrapper over parsed form fields with repeated-value support."""

    def __init__(self, values: dict[str, list[str]]) -> None:
        self.values = values

    @classmethod
    def from_request(cls, req: Any) -> "Form":
        try:
            raw = req.body.decode("utf-8")
        except UnicodeDecodeError:
            raw = req.body.decode("latin-1")
        return cls(parse_qs(raw, keep_blank_values=True))

    def get(self, name: str, default: str = "") -> str:
        values = self.values.get(name)
        if not values:
            return default
        return values[-1]

    def getlist(self, name: str) -> list[str]:
        return self.values.get(name, [])

    def present(self) -> dict[str, str]:
        return {key: values[-1] for key, values in self.values.items() if values}


class UIApp:
    def __init__(self, service: InvoiceService) -> None:
        self.service = service
        self._env = None
        if Environment is not None:
            self._env = Environment(
                loader=FileSystemLoader(str(TEMPLATES_DIR)),
                autoescape=select_autoescape(("html", "j2")),
                trim_blocks=True,
                lstrip_blocks=True,
            )
            self._env.filters["status_label"] = lambda value: str(value).replace("_", " ").title()
            self._env.filters["initials"] = _initials
            self._env.filters["short_date"] = _short_date

    # ------------------------------------------------------------------
    # Registration
    # ------------------------------------------------------------------

    def register(self, add: Callable[[str, str, Handler], None]) -> None:
        add("GET", r"^/ui$", self._guard(self.dashboard))
        add("GET", r"^/ui/invoices$", self._guard(self.invoices))
        add("GET", r"^/ui/invoices/new$", self._guard(self.invoice_new))
        add("POST", r"^/ui/invoices$", self._guard(self.invoice_create))
        add("GET", r"^/ui/invoices/(?P<id>[^/]+)/edit$", self._guard(self.invoice_edit))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)$", self._guard(self.invoice_update))
        add("GET", r"^/ui/invoices/(?P<id>[^/]+)$", self._guard(self.invoice_detail))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)/issue$", self._guard(self.invoice_issue))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)/send$", self._guard(self.invoice_send))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)/void$", self._guard(self.invoice_void))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)/delete$", self._guard(self.invoice_delete))
        add("POST", r"^/ui/invoices/(?P<id>[^/]+)/payments$", self._guard(self.payment_create))
        add(
            "POST",
            r"^/ui/invoices/(?P<id>[^/]+)/payments/(?P<payment_id>[^/]+)/void$",
            self._guard(self.payment_void),
        )
        add("GET", r"^/ui/invoices/(?P<id>[^/]+)/print$", self._guard(self.invoice_print))
        add("GET", r"^/ui/customers$", self._guard(self.customers))
        add("GET", r"^/ui/customers/new$", self._guard(self.customer_new))
        add("POST", r"^/ui/customers$", self._guard(self.customer_create))
        add("GET", r"^/ui/customers/(?P<id>[^/]+)/edit$", self._guard(self.customer_edit))
        add("POST", r"^/ui/customers/(?P<id>[^/]+)$", self._guard(self.customer_update))
        add("GET", r"^/ui/customers/(?P<id>[^/]+)$", self._guard(self.customer_detail))
        add("POST", r"^/ui/customers/(?P<id>[^/]+)/archive$", self._guard(self.customer_archive))
        add("POST", r"^/ui/customers/(?P<id>[^/]+)/restore$", self._guard(self.customer_restore))
        add("GET", r"^/static/(?P<path>.+)$", self.static)

    def _guard(self, handler: Handler) -> Handler:
        @wraps(handler)
        async def guarded(req: Any, match: Any) -> Page:
            try:
                return await handler(req, match)
            except ApiError as err:
                return self._error_page(req, err)
            except Exception:
                logger.exception("unhandled UI error path=%s", getattr(req, "path", "?"))
                return self._error_page(
                    req,
                    ApiError("internal server error", code="internal_error", status=500),
                )

        return guarded

    # ------------------------------------------------------------------
    # Dashboard
    # ------------------------------------------------------------------

    async def dashboard_safe(self, req: Any, match: Any) -> Page:
        return await self._guard(self.dashboard)(req, match)

    async def dashboard(self, req: Any, _match: Any) -> Page:
        summary = await self.service.summary()
        recent = await self.service.list_invoices(limit=8)
        customers = await self.service.list_customers(status="active", limit=5)
        overdue = await self.service.list_invoices(overdue=True, limit=1)
        return self._render(
            "dashboard.html.j2",
            req,
            active="dashboard",
            summary=summary,
            recent=recent,
            customers=customers,
            overdue_count=overdue["total"],
        )

    # ------------------------------------------------------------------
    # Invoices
    # ------------------------------------------------------------------

    async def invoices(self, req: Any, _match: Any) -> Page:
        status = _query(req, "status") or None
        customer_id = _query(req, "customer_id") or None
        search = _query(req, "q") or None
        page_no = _positive_int(_query(req, "page"), 1)
        limit = 25
        result = await self.service.list_invoices(
            status=status,
            customer_id=customer_id,
            q=search,
            limit=limit,
            offset=(page_no - 1) * limit,
        )
        customers = await self.service.list_customers(status="all", limit=200)
        page_count = max(1, (result["total"] + limit - 1) // limit)
        filters = {
            "status": status or "all",
            "customer_id": customer_id or "",
            "q": search or "",
        }
        return self._render(
            "invoices.html.j2",
            req,
            active="invoices",
            result=result,
            customers=customers["items"],
            filters=filters,
            page_no=page_no,
            page_count=page_count,
            prev_url=_page_url(self._base() + "/ui/invoices", page_no - 1, filters) if page_no > 1 else None,
            next_url=(
                _page_url(self._base() + "/ui/invoices", page_no + 1, filters)
                if page_no < page_count
                else None
            ),
        )

    async def invoice_new(self, req: Any, _match: Any) -> Page:
        customers = await self.service.list_customers(status="active", limit=200)
        today = self.service.clock().date()
        form = {
            "customer_id": _query(req, "customer_id") or "",
            "issue_date": today.isoformat(),
            "due_date": (today + timedelta(days=14)).isoformat(),
            "notes": "",
            "terms": "Payment is due within the agreed terms.",
        }
        return self._render(
            "invoice_form.html.j2",
            req,
            active="invoices",
            mode="create",
            invoice=None,
            customers=customers["items"],
            form=form,
            line_items=[{"description": "", "quantity": "1", "unit_price": "", "tax": "", "discount": ""}],
            error=None,
        )

    async def invoice_create(self, req: Any, _match: Any) -> Page:
        form = Form.from_request(req)
        invoice_id = None
        try:
            payload = _invoice_payload(form)
            invoice = await self.service.create_invoice(payload)
            invoice_id = invoice["id"]
        except ApiError as err:
            return await self._invoice_form_error(req, form, err)
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_edit(self, req: Any, match: Any) -> Page:
        invoice = await self.service.get_invoice(match.group("id"))
        if invoice["status"] != m.DRAFT:
            raise ValidationError("only draft invoices can be edited", code="invoice_not_draft")
        customers = await self.service.list_customers(status="active", limit=200)
        form = {
            "customer_id": invoice["customer_id"],
            "issue_date": invoice["issue_date"],
            "due_date": invoice["due_date"],
            "notes": invoice["notes"] or "",
            "terms": invoice["terms"] or "",
        }
        line_items = [
            {
                "description": item["description"],
                "quantity": item["quantity"],
                "unit_price": item["unit_price"],
                "tax": _bp_to_percent(item["tax_rate_bp"]),
                "discount": _bp_to_percent(item["discount_bp"]),
            }
            for item in invoice["line_items"]
        ] or [{"description": "", "quantity": "1", "unit_price": "", "tax": "", "discount": ""}]
        return self._render(
            "invoice_form.html.j2",
            req,
            active="invoices",
            mode="edit",
            invoice=invoice,
            customers=customers["items"],
            form=form,
            line_items=line_items,
            error=None,
        )

    async def invoice_update(self, req: Any, match: Any) -> Page:
        form = Form.from_request(req)
        invoice_id = match.group("id")
        try:
            payload = _invoice_payload(form)
            payload.pop("customer_id", None)
            await self.service.update_invoice(invoice_id, payload)
        except ApiError as err:
            invoice = None
            try:
                invoice = await self.service.get_invoice(invoice_id)
            except ApiError:
                pass
            return await self._invoice_form_error(
                req, form, err, mode="edit", invoice=invoice, invoice_id=invoice_id
            )
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_detail(self, req: Any, match: Any) -> Page:
        return await self._invoice_page(req, match.group("id"))

    async def _invoice_page(
        self, req: Any, invoice_id: str, *, error: ApiError | None = None, form: dict[str, str] | None = None
    ) -> Page:
        invoice = await self.service.get_invoice(invoice_id)
        payments = await self.service.list_payments(invoice_id)
        customer = invoice.get("customer")
        try:
            customer = await self.service.get_customer(invoice["customer_id"])
        except ApiError:
            pass
        active_payments = [item for item in payments["items"] if not item["voided_at"]]
        can_void = invoice["status"] in m.PAYABLE_STATUSES and not active_payments
        can_pay = (
            invoice["status"] in m.PAYABLE_STATUSES
            and Decimal(invoice["totals"]["balance_due"]) > 0
        )
        return self._render(
            "invoice.html.j2",
            req,
            status=error.status if error else 200,
            active="invoices",
            invoice=invoice,
            customer=customer or {},
            payments=payments["items"],
            can_void=can_void,
            can_pay=can_pay,
            error=error,
            form=form or {},
        )

    async def invoice_issue(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        try:
            await self.service.issue_invoice(invoice_id, {})
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err)
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_send(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        try:
            await self.service.send_invoice(invoice_id)
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err)
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_void(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        form = Form.from_request(req)
        try:
            await self.service.void_invoice(invoice_id, {"reason": form.get("reason") or None})
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err)
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_delete(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        try:
            await self.service.delete_invoice(invoice_id)
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err)
        return redirect(f"{self._base()}/ui/invoices")

    async def payment_create(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        form = Form.from_request(req)
        payload = {
            "amount": form.get("amount"),
            "method": form.get("method") or "bank_transfer",
            "reference": form.get("reference") or None,
        }
        if form.get("paid_at"):
            payload["paid_at"] = form.get("paid_at")
        try:
            await self.service.record_payment(invoice_id, payload)
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err, form=form.present())
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def payment_void(self, req: Any, match: Any) -> Page:
        invoice_id = match.group("id")
        try:
            await self.service.void_payment(invoice_id, match.group("payment_id"))
        except ApiError as err:
            return await self._invoice_page(req, invoice_id, error=err)
        return redirect(f"{self._base()}/ui/invoices/{invoice_id}")

    async def invoice_print(self, req: Any, match: Any) -> Page:
        rendered = await self.service.render_invoice(match.group("id"))
        return html(rendered["html"])

    async def _invoice_form_error(
        self,
        req: Any,
        form: Form,
        err: ApiError,
        *,
        mode: str = "create",
        invoice: dict[str, Any] | None = None,
        invoice_id: str | None = None,
    ) -> Page:
        customers = await self.service.list_customers(status="active", limit=200)
        values = form.present()
        if invoice is None and invoice_id:
            try:
                invoice = await self.service.get_invoice(invoice_id)
            except ApiError:
                invoice = None
        return self._render(
            "invoice_form.html.j2",
            req,
            active="invoices",
            mode=mode,
            invoice=invoice,
            customers=customers["items"],
            form=values,
            line_items=_form_line_items(form),
            error=err,
        )

    # ------------------------------------------------------------------
    # Customers
    # ------------------------------------------------------------------

    async def customers(self, req: Any, _match: Any) -> Page:
        status = _query(req, "status") or "active"
        search = _query(req, "q") or None
        page_no = _positive_int(_query(req, "page"), 1)
        limit = 25
        if status not in ("active", "archived", "all"):
            status = "active"
        result = await self.service.list_customers(
            status=status,
            q=search,
            limit=limit,
            offset=(page_no - 1) * limit,
        )
        page_count = max(1, (result["total"] + limit - 1) // limit)
        filters = {"status": status, "q": search or ""}
        return self._render(
            "customers.html.j2",
            req,
            active="customers",
            result=result,
            filters=filters,
            page_no=page_no,
            page_count=page_count,
            prev_url=_page_url(self._base() + "/ui/customers", page_no - 1, filters) if page_no > 1 else None,
            next_url=(
                _page_url(self._base() + "/ui/customers", page_no + 1, filters)
                if page_no < page_count
                else None
            ),
        )

    async def customer_new(self, req: Any, _match: Any) -> Page:
        form = {
            "name": "",
            "email": "",
            "phone": "",
            "tax_id": "",
            "currency": "USD",
            "payment_terms_days": "14",
            "line1": "",
            "line2": "",
            "city": "",
            "state": "",
            "postal_code": "",
            "country": "",
        }
        return self._render(
            "customer_form.html.j2",
            req,
            active="customers",
            mode="create",
            customer=None,
            form=form,
            error=None,
        )

    async def customer_create(self, req: Any, _match: Any) -> Page:
        form = Form.from_request(req)
        try:
            customer = await self.service.create_customer(_customer_payload(form))
        except ApiError as err:
            return self._render(
                "customer_form.html.j2",
                req,
                active="customers",
                mode="create",
                customer=None,
                form=form.present(),
                error=err,
            )
        return redirect(f"{self._base()}/ui/customers/{customer['id']}")

    async def customer_edit(self, req: Any, match: Any) -> Page:
        customer = await self.service.get_customer(match.group("id"))
        address = customer.get("address") or {}
        form = {
            "name": customer["name"],
            "email": customer.get("email") or "",
            "phone": customer.get("phone") or "",
            "tax_id": customer.get("tax_id") or "",
            "currency": customer["currency"],
            "payment_terms_days": str(customer["payment_terms_days"]),
            "line1": address.get("line1", ""),
            "line2": address.get("line2", ""),
            "city": address.get("city", ""),
            "state": address.get("state", ""),
            "postal_code": address.get("postal_code", ""),
            "country": address.get("country", ""),
        }
        return self._render(
            "customer_form.html.j2",
            req,
            active="customers",
            mode="edit",
            customer=customer,
            form=form,
            error=None,
        )

    async def customer_update(self, req: Any, match: Any) -> Page:
        form = Form.from_request(req)
        customer_id = match.group("id")
        try:
            await self.service.update_customer(customer_id, _customer_payload(form))
        except ApiError as err:
            customer = None
            try:
                customer = await self.service.get_customer(customer_id)
            except ApiError:
                pass
            return self._render(
                "customer_form.html.j2",
                req,
                active="customers",
                mode="edit",
                customer=customer,
                form=form.present(),
                error=err,
            )
        return redirect(f"{self._base()}/ui/customers/{customer_id}")

    async def customer_detail(self, req: Any, match: Any) -> Page:
        customer_id = match.group("id")
        customer = await self.service.get_customer(customer_id)
        invoices = await self.service.list_invoices(customer_id=customer_id, limit=100)
        return self._render(
            "customer.html.j2",
            req,
            active="customers",
            customer=customer,
            result=invoices,
            error=None,
        )

    async def customer_archive(self, req: Any, match: Any) -> Page:
        customer_id = match.group("id")
        try:
            await self.service.archive_customer(customer_id)
        except ApiError as err:
            customer = await self.service.get_customer(customer_id)
            invoices = await self.service.list_invoices(customer_id=customer_id, limit=100)
            return self._render(
                "customer.html.j2",
                req,
                active="customers",
                customer=customer,
                result=invoices,
                error=err,
            )
        return redirect(f"{self._base()}/ui/customers/{customer_id}")

    async def customer_restore(self, req: Any, match: Any) -> Page:
        customer_id = match.group("id")
        await self.service.restore_customer(customer_id)
        return redirect(f"{self._base()}/ui/customers/{customer_id}")

    # ------------------------------------------------------------------
    # Static files
    # ------------------------------------------------------------------

    async def static(self, _req: Any, match: Any) -> Page:
        relative = match.group("path")
        candidate = (STATIC_DIR / relative).resolve()
        try:
            candidate.relative_to(STATIC_DIR.resolve())
        except ValueError:
            return Page(b"not found", status=404, content_type="text/plain; charset=utf-8")
        if not candidate.is_file():
            return Page(b"not found", status=404, content_type="text/plain; charset=utf-8")
        content_type = _CONTENT_TYPES.get(candidate.suffix.lower(), "application/octet-stream")
        return Page(
            candidate.read_bytes(),
            content_type=content_type,
            headers={"Cache-Control": "public, max-age=300"},
        )

    # ------------------------------------------------------------------
    # Rendering
    # ------------------------------------------------------------------

    def _base(self) -> str:
        explicit = os.environ.get("INVOICES_UI_BASE")
        if explicit is not None:
            return explicit.rstrip("/")
        project = os.environ.get("PROJECT", "").strip()
        if not project:
            return ""
        name = os.environ.get("INVOICES_FUNCTION_NAME", "invoices").strip() or "invoices"
        return f"/lambda/{project}/{name}"

    def _render(self, template: str, req: Any, *, status: int = 200, **context: Any) -> Page:
        if self._env is None:
            return Page(
                body=_MISSING_JINJA_PAGE,
                status=503,
                content_type="text/html; charset=utf-8",
            )
        base = self._base()
        body = self._env.get_template(template).render(
            base=base,
            current_path=getattr(req, "path", ""),
            **context,
        )
        return html(body, status=status)

    def _error_page(self, req: Any, err: ApiError) -> Page:
        status = err.status if 400 <= err.status <= 599 else 500
        if self._env is None:
            return html(_MISSING_JINJA_PAGE, status=503)
        body = self._env.get_template("error.html.j2").render(
            base=self._base(),
            current_path=getattr(req, "path", ""),
            error=err,
            status=status,
        )
        return html(body, status=status)


# ----------------------------------------------------------------------
# Form parsing
# ----------------------------------------------------------------------


def _invoice_payload(form: Form) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "customer_id": form.get("customer_id"),
        "issue_date": form.get("issue_date") or None,
        "due_date": form.get("due_date") or None,
        "notes": form.get("notes") or None,
        "terms": form.get("terms") or None,
        "line_items": _form_line_items(form),
    }
    return {key: value for key, value in payload.items() if value not in (None, "")}


def _form_line_items(form: Form) -> list[dict[str, Any]]:
    descriptions = form.getlist("line_description")
    quantities = form.getlist("line_quantity")
    prices = form.getlist("line_unit_price")
    taxes = form.getlist("line_tax")
    discounts = form.getlist("line_discount")
    items: list[dict[str, Any]] = []
    for index, description in enumerate(descriptions):
        raw = {
            "description": description,
            "quantity": _at(quantities, index),
            "unit_price": _at(prices, index),
            "tax": _at(taxes, index),
            "discount": _at(discounts, index),
        }
        if not any(value.strip() for value in raw.values()):
            continue
        items.append(
            {
                "description": raw["description"].strip(),
                "quantity": raw["quantity"] or "1",
                "unit_price": raw["unit_price"] or "0",
                "tax_rate_bp": _percent_to_bp(raw["tax"], f"line_items[{index}].tax_rate_bp"),
                "discount_bp": _percent_to_bp(raw["discount"], f"line_items[{index}].discount_bp"),
            }
        )
    return items


def _customer_payload(form: Form) -> dict[str, Any]:
    address = {
        "line1": form.get("line1"),
        "line2": form.get("line2"),
        "city": form.get("city"),
        "state": form.get("state"),
        "postal_code": form.get("postal_code"),
        "country": form.get("country"),
    }
    payload: dict[str, Any] = {
        "name": form.get("name"),
        "email": form.get("email") or None,
        "phone": form.get("phone") or None,
        "tax_id": form.get("tax_id") or None,
        "currency": form.get("currency") or "USD",
        "address": {key: value for key, value in address.items() if value} or None,
    }
    terms = form.get("payment_terms_days").strip()
    if terms:
        try:
            payload["payment_terms_days"] = int(terms)
        except ValueError as exc:
            raise ValidationError("payment_terms_days must be an integer") from exc
    return payload


def _percent_to_bp(value: str, field: str) -> int:
    raw = (value or "").strip()
    if not raw:
        return 0
    try:
        percent = Decimal(raw)
    except InvalidOperation as exc:
        raise ValidationError(f"{field} must be a percentage") from exc
    if percent < 0 or percent > 100:
        raise ValidationError(f"{field} must be between 0 and 100")
    return int((percent * 100).to_integral_value(rounding=ROUND_HALF_UP))


def _bp_to_percent(value: int) -> str:
    percent = (Decimal(value) / 100).normalize()
    return f"{percent:f}"


def _at(values: list[str], index: int) -> str:
    if index < len(values):
        return values[index]
    return ""


# ----------------------------------------------------------------------
# Query helpers and filters
# ----------------------------------------------------------------------


def _query(req: Any, name: str) -> str | None:
    value = req.query.get(name)
    if value is None:
        return None
    value = value.strip()
    return value or None


def _positive_int(value: str | None, default: int) -> int:
    if value is None:
        return default
    try:
        parsed = int(value)
    except ValueError:
        return default
    return parsed if parsed > 0 else default


def _initials(value: str) -> str:
    words = [word for word in str(value).split() if word]
    if not words:
        return "?"
    if len(words) == 1:
        return words[0][:2].upper()
    return (words[0][0] + words[-1][0]).upper()


def _short_date(value: str | None) -> str:
    if not value:
        return "—"
    try:
        return date.fromisoformat(str(value)[:10]).strftime("%b %d, %Y")
    except ValueError:
        return str(value)


def _page_url(path: str, page_no: int, filters: dict[str, Any]) -> str:
    params = {key: value for key, value in filters.items() if value not in (None, "")}
    if page_no > 1:
        params["page"] = page_no
    return f"{path}?{urlencode(params)}" if params else path


_MISSING_JINJA_PAGE = """<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Invoice UI unavailable</title></head>
<body style="font-family:system-ui,sans-serif;max-width:640px;margin:80px auto;line-height:1.5">
<h1>Invoice UI unavailable</h1>
<p>The UI needs <code>jinja2</code>. The LiteFunctions Python runtime includes it;
for local development install it with:</p>
<pre><code>python3 -m pip install jinja2</code></pre>
<p>Then restart the dev server. The JSON API keeps working without it.</p>
</body></html>"""
