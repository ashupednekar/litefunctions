"""Transport adapter: sync HTTP requests and async op envelopes.

Both transports produce the same envelope:

    {"ok": true,  "status": 200, "data": ...}
    {"ok": false, "status": 422, "error": {"code": ..., "message": ...}}

The LiteFunctions runtime always returns HTTP 200 for sync calls, so `status`
inside the envelope is the authoritative status code.
"""

from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any, Awaitable, Callable

from .errors import ApiError, ValidationError
from .service import InvoiceService

logger = logging.getLogger(__name__)

Handler = Callable[["Request", Any], Awaitable[dict[str, Any]]]


@dataclass
class Request:
    method: str
    path: str
    query: dict[str, str] = field(default_factory=dict)
    headers: dict[str, str] = field(default_factory=dict)
    body: bytes = b""

    def header(self, name: str) -> str | None:
        return self.headers.get(name.lower())

    def json(self) -> dict[str, Any]:
        if not self.body:
            return {}
        try:
            payload = json.loads(self.body.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ValidationError("request body must be valid JSON") from exc
        if not isinstance(payload, dict):
            raise ValidationError("request body must be a JSON object")
        return payload


@dataclass
class _Route:
    method: str
    pattern: re.Pattern[str]
    handler: Handler
    kind: str = "json"


class HttpApp:
    def __init__(self, service: InvoiceService) -> None:
        self.service = service
        self._routes: list[_Route] = []
        self._register()

    # ------------------------------------------------------------------
    # Public entrypoints
    # ------------------------------------------------------------------

    async def from_starlette(self, request: Any) -> dict[str, Any]:
        body = await request.body()
        query = {key: value for key, value in request.query_params.items()}
        headers = {key.lower(): value for key, value in request.headers.items()}
        payload = Request(
            method=request.method.upper(),
            path=request.url.path,
            query=query,
            headers=headers,
            body=body,
        )

        # The ingestor proxy forwards only `/lambda/<project>/<name>`, so the
        # runtime always sees path `/`. Support an op envelope in that case:
        #   GET  .../invoices?op=list_invoices&status=open
        #   POST .../invoices  {"op":"create_customer","name":"Acme"}
        parsed: dict[str, Any] | None = None
        if body:
            try:
                candidate = json.loads(body.decode("utf-8"))
            except (UnicodeDecodeError, json.JSONDecodeError):
                candidate = None
            if isinstance(candidate, dict):
                parsed = candidate

        op = headers.get("x-litefunction-op") or query.get("op")
        if isinstance(parsed, dict) and isinstance(parsed.get("op"), str):
            op = parsed["op"]

        path = payload.path.rstrip("/") or "/"
        if op:
            envelope: dict[str, Any] = dict(parsed or {})
            envelope["op"] = op
            if headers:
                envelope["headers"] = headers
            extra = {key: value for key, value in query.items() if key != "op"}
            if extra:
                merged = envelope.get("query")
                if not isinstance(merged, dict):
                    merged = {}
                for key, value in extra.items():
                    merged.setdefault(key, value)
                envelope["query"] = merged
            return await self.from_op(envelope)

        if path == "/" and payload.method == "GET":
            return await self.handle(payload)

        if path == "/":
            return self._error(
                ApiError(
                    "pass ?op=<operation> or include an 'op' field in the JSON body",
                    code="missing_op",
                    status=422,
                    details={"known_ops": sorted(OPS)},
                )
            )

        return await self.handle(payload)

    async def from_op(self, payload: dict[str, Any]) -> dict[str, Any]:
        try:
            request = self._op_request(payload)
        except ApiError as err:
            return self._error(err)
        return await self.handle(request)

    async def handle(self, request: Request) -> dict[str, Any]:
        path = request.path or "/"
        if len(path) > 1:
            path = path.rstrip("/")

        allowed: set[str] = set()
        for route in self._routes:
            match = route.pattern.search(path)
            if not match:
                continue
            if route.method != request.method.upper():
                allowed.add(route.method)
                continue
            try:
                data = await route.handler(request, match)
            except ApiError as err:
                return self._error(err)
            except Exception:
                logger.exception(
                    "unhandled error method=%s path=%s", request.method, request.path
                )
                return self._error(
                    ApiError("internal server error", code="internal_error", status=500)
                )
            return {
                "ok": True,
                "status": 201 if request.method.upper() == "POST" and data.get("__created__") else 200,
                "data": {key: value for key, value in data.items() if key != "__created__"},
            }

        if allowed:
            return self._error(
                ApiError(
                    f"method {request.method} is not allowed for this path",
                    code="method_not_allowed",
                    status=405,
                    details={"allowed": sorted(allowed)},
                )
            )
        return self._error(
            ApiError(
                f"no route for {request.method} {request.path}",
                code="not_found",
                status=404,
            )
        )

    # ------------------------------------------------------------------
    # Routes
    # ------------------------------------------------------------------

    def _register(self) -> None:
        s = self.service

        async def health(_req: Request, _match: Any) -> dict[str, Any]:
            return {
                "service": "invoices",
                "status": "ok",
                "backend": type(s.store).__name__,
            }

        async def create_customer(req: Request, _match: Any) -> dict[str, Any]:
            doc = await s.create_customer(req.json(), req.header("idempotency-key"))
            return {**doc, "__created__": True}

        async def list_customers(req: Request, _match: Any) -> dict[str, Any]:
            return await s.list_customers(
                status=_query(req, "status"),
                q=_query(req, "q"),
                limit=_limit(req),
                offset=_offset(req),
            )

        async def get_customer(_req: Request, match: Any) -> dict[str, Any]:
            return await s.get_customer(match.group("id"))

        async def update_customer(req: Request, match: Any) -> dict[str, Any]:
            return await s.update_customer(match.group("id"), req.json())

        async def archive_customer(_req: Request, match: Any) -> dict[str, Any]:
            return await s.archive_customer(match.group("id"))

        async def restore_customer(_req: Request, match: Any) -> dict[str, Any]:
            return await s.restore_customer(match.group("id"))

        async def create_invoice(req: Request, _match: Any) -> dict[str, Any]:
            doc = await s.create_invoice(req.json(), req.header("idempotency-key"))
            return {**doc, "__created__": True}

        async def list_invoices(req: Request, _match: Any) -> dict[str, Any]:
            return await s.list_invoices(
                status=_query(req, "status"),
                customer_id=_query(req, "customer_id"),
                q=_query(req, "q"),
                overdue=_bool_query(req, "overdue"),
                limit=_limit(req),
                offset=_offset(req),
            )

        async def get_invoice(_req: Request, match: Any) -> dict[str, Any]:
            return await s.get_invoice(match.group("id"))

        async def update_invoice(req: Request, match: Any) -> dict[str, Any]:
            return await s.update_invoice(match.group("id"), req.json())

        async def delete_invoice(_req: Request, match: Any) -> dict[str, Any]:
            return await s.delete_invoice(match.group("id"))

        async def issue_invoice(req: Request, match: Any) -> dict[str, Any]:
            return await s.issue_invoice(match.group("id"), req.json())

        async def void_invoice(req: Request, match: Any) -> dict[str, Any]:
            return await s.void_invoice(match.group("id"), req.json())

        async def send_invoice(_req: Request, match: Any) -> dict[str, Any]:
            return await s.send_invoice(match.group("id"))

        async def record_payment(req: Request, match: Any) -> dict[str, Any]:
            doc = await s.record_payment(
                match.group("id"), req.json(), req.header("idempotency-key")
            )
            return {**doc, "__created__": True}

        async def list_payments(_req: Request, match: Any) -> dict[str, Any]:
            return await s.list_payments(match.group("id"))

        async def void_payment(req: Request, match: Any) -> dict[str, Any]:
            return await s.void_payment(
                match.group("id"), match.group("payment_id"), req.json()
            )

        async def render_invoice(_req: Request, match: Any) -> dict[str, Any]:
            return await s.render_invoice(match.group("id"))

        async def summary(_req: Request, _match: Any) -> dict[str, Any]:
            return await s.summary()

        self._add("GET", r"/health$", health)
        self._add("GET", r"^/?$", health)
        self._add("POST", r"/customers$", create_customer)
        self._add("GET", r"/customers$", list_customers)
        self._add("GET", r"/customers/(?P<id>[^/]+)$", get_customer)
        self._add("PATCH", r"/customers/(?P<id>[^/]+)$", update_customer)
        self._add("POST", r"/customers/(?P<id>[^/]+)/archive$", archive_customer)
        self._add("POST", r"/customers/(?P<id>[^/]+)/restore$", restore_customer)
        self._add("DELETE", r"/customers/(?P<id>[^/]+)$", archive_customer)
        self._add("POST", r"/invoices$", create_invoice)
        self._add("GET", r"/invoices$", list_invoices)
        self._add("GET", r"/invoices/(?P<id>[^/]+)$", get_invoice)
        self._add("PATCH", r"/invoices/(?P<id>[^/]+)$", update_invoice)
        self._add("DELETE", r"/invoices/(?P<id>[^/]+)$", delete_invoice)
        self._add("POST", r"/invoices/(?P<id>[^/]+)/issue$", issue_invoice)
        self._add("POST", r"/invoices/(?P<id>[^/]+)/void$", void_invoice)
        self._add("POST", r"/invoices/(?P<id>[^/]+)/send$", send_invoice)
        self._add("GET", r"/invoices/(?P<id>[^/]+)/render$", render_invoice)
        self._add("POST", r"/invoices/(?P<id>[^/]+)/payments$", record_payment)
        self._add("GET", r"/invoices/(?P<id>[^/]+)/payments$", list_payments)
        self._add(
            "POST", r"/invoices/(?P<id>[^/]+)/payments/(?P<payment_id>[^/]+)/void$", void_payment
        )
        self._add("GET", r"/summary$", summary)

    def _add(self, method: str, pattern: str, handler: Handler) -> None:
        self._routes.append(_Route(method=method, pattern=re.compile(pattern), handler=handler))

    # ------------------------------------------------------------------
    # Async op mapping
    # ------------------------------------------------------------------

    def _op_request(self, payload: Any) -> Request:
        if not isinstance(payload, dict):
            raise ValidationError("async payload must be a JSON object")
        op = payload.get("op") or payload.get("operation") or payload.get("action")
        if not isinstance(op, str) or not op.strip():
            raise ValidationError("async payload requires an 'op' field")
        op = _snake(op.strip())
        spec = OPS.get(op)
        if spec is None:
            raise ValidationError(
                f"unknown op '{op}'", details={"known_ops": sorted(OPS)}
            )
        method, path = spec

        ids: dict[str, str] = {}
        if "{id}" in path:
            value = (
                payload.get("id")
                or payload.get("invoice_id")
                or payload.get("customer_id")
                or _slot(payload, "id")
                or _slot(payload, "invoice_id")
            )
            if not isinstance(value, str) or not value:
                raise ValidationError(f"op '{op}' requires 'id'")
            ids["id"] = value
        if "{payment_id}" in path:
            value = payload.get("payment_id") or _slot(payload, "payment_id")
            if not isinstance(value, str) or not value:
                raise ValidationError(f"op '{op}' requires 'payment_id'")
            ids["payment_id"] = value

        body = payload.get("input")
        if body is None:
            body = payload.get("payload")
        if body is None:
            reserved = {
                "op",
                "operation",
                "action",
                "query",
                "headers",
                "idempotency_key",
                "id",
                "invoice_id",
                "payment_id",
            }
            body = {key: value for key, value in payload.items() if key not in reserved}
        if not isinstance(body, dict):
            raise ValidationError("'input' must be an object")

        query: dict[str, str] = {}
        raw_query = payload.get("query")
        if raw_query is not None:
            if not isinstance(raw_query, dict):
                raise ValidationError("'query' must be an object")
            query.update({str(key): str(value) for key, value in raw_query.items()})
        if method == "GET":
            for key in ("status", "q", "limit", "offset", "overdue", "customer_id"):
                if key in payload and key not in query:
                    query[key] = str(payload[key])

        headers: dict[str, str] = {}
        raw_headers = payload.get("headers")
        if isinstance(raw_headers, dict):
            headers = {str(key).lower(): str(value) for key, value in raw_headers.items()}
        if isinstance(payload.get("idempotency_key"), str):
            headers["idempotency-key"] = payload["idempotency_key"]

        return Request(
            method=method,
            path=path.format(**ids),
            query=query,
            headers=headers,
            body=json.dumps(body).encode("utf-8") if body else b"",
        )

    def _error(self, err: ApiError) -> dict[str, Any]:
        return {"ok": False, "status": err.status, "error": err.to_dict()}


OPS: dict[str, tuple[str, str]] = {
    # customers
    "health": ("GET", "/health"),
    "create_customer": ("POST", "/customers"),
    "get_customer": ("GET", "/customers/{id}"),
    "list_customers": ("GET", "/customers"),
    "update_customer": ("PATCH", "/customers/{id}"),
    "archive_customer": ("POST", "/customers/{id}/archive"),
    "restore_customer": ("POST", "/customers/{id}/restore"),
    # invoices
    "create_invoice": ("POST", "/invoices"),
    "get_invoice": ("GET", "/invoices/{id}"),
    "list_invoices": ("GET", "/invoices"),
    "update_invoice": ("PATCH", "/invoices/{id}"),
    "delete_invoice": ("DELETE", "/invoices/{id}"),
    "issue_invoice": ("POST", "/invoices/{id}/issue"),
    "void_invoice": ("POST", "/invoices/{id}/void"),
    "send_invoice": ("POST", "/invoices/{id}/send"),
    "render_invoice": ("GET", "/invoices/{id}/render"),
    # payments
    "record_payment": ("POST", "/invoices/{id}/payments"),
    "list_payments": ("GET", "/invoices/{id}/payments"),
    "void_payment": ("POST", "/invoices/{id}/payments/{payment_id}/void"),
    # reporting
    "summary": ("GET", "/summary"),
}


def _snake(value: str) -> str:
    value = value.replace("-", "_").replace(".", "_").replace(" ", "_")
    return re.sub(r"(?<!^)(?=[A-Z])", "_", value).lower()


def _slot(payload: dict[str, Any], name: str) -> str | None:
    query = payload.get("query")
    if isinstance(query, dict):
        value = query.get(name)
        if isinstance(value, str):
            return value
    return None


def _query(req: Request, name: str) -> str | None:
    value = req.query.get(name)
    if value is None:
        return None
    value = value.strip()
    return value or None


def _limit(req: Request) -> int:
    raw = _query(req, "limit")
    if raw is None:
        return 50
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValidationError("limit must be an integer") from exc
    if not 1 <= value <= 200:
        raise ValidationError("limit must be between 1 and 200")
    return value


def _offset(req: Request) -> int:
    raw = _query(req, "offset")
    if raw is None:
        return 0
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValidationError("offset must be an integer") from exc
    if value < 0:
        raise ValidationError("offset must be zero or greater")
    return value


def _bool_query(req: Request, name: str) -> bool | None:
    raw = _query(req, name)
    if raw is None:
        return None
    lowered = raw.lower()
    if lowered in ("1", "true", "yes"):
        return True
    if lowered in ("0", "false", "no"):
        return False
    raise ValidationError(f"{name} must be true or false")
