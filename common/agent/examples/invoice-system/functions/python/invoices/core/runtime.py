"""Runtime wiring: one NATS connection and one service per function process."""

from __future__ import annotations

import asyncio
import json
import logging
import os
from typing import Any

from .errors import ApiError, StoreUnavailableError
from .service import InvoiceService
from .store import NatsStore

logger = logging.getLogger(__name__)

_lock: asyncio.Lock | None = None
_service: InvoiceService | None = None
_store: NatsStore | None = None


def _get_lock() -> asyncio.Lock:
    global _lock
    if _lock is None:
        _lock = asyncio.Lock()
    return _lock


async def get_service() -> InvoiceService:
    global _service, _store
    if _service is not None:
        return _service

    async with _get_lock():
        if _service is not None:
            return _service

        url = os.environ.get("NATS_URL", "").strip()
        if not url:
            raise StoreUnavailableError(
                "NATS_URL is not set for this runtime", code="nats_url_missing"
            )
        project = os.environ.get("PROJECT", "default") or "default"
        prefix = os.environ.get("INVOICES_KV_PREFIX", f"{project}_invoices")
        subject_prefix = os.environ.get(
            "INVOICES_EVENTS_SUBJECT", f"{project}.invoices.events"
        )

        store = await NatsStore.connect(url, prefix=prefix)
        publisher = EventPublisher(store.nc, subject_prefix)
        _store = store
        _service = InvoiceService(store, publisher=publisher.publish)
        logger.info(
            "invoice service ready project=%s nats=%s kv_prefix=%s events=%s",
            project,
            _redact_url(url),
            prefix,
            subject_prefix,
        )
        return _service


class EventPublisher:
    """Best-effort domain events on core NATS; failures never fail a request."""

    def __init__(self, nc: Any, subject_prefix: str) -> None:
        self.nc = nc
        self.subject_prefix = subject_prefix.rstrip(".")

    async def publish(self, event: str, payload: dict[str, Any]) -> None:
        subject = f"{self.subject_prefix}.{event}"
        body = json.dumps(
            {"event": event, "data": payload}, separators=(",", ":"), default=str
        ).encode("utf-8")
        await self.nc.publish(subject, body)
        try:
            await self.nc.flush(timeout=2)
        except Exception:
            logger.warning("event flush timed out subject=%s", subject)


async def reset() -> None:
    """Test helper / graceful shutdown hook."""
    global _service, _store
    if _store is not None:
        await _store.close()
    _service = None
    _store = None


def error_envelope(err: ApiError) -> dict[str, Any]:
    return {"ok": False, "status": err.status, "error": err.to_dict()}


def _redact_url(value: str) -> str:
    if "@" in value:
        scheme, rest = value.split("://", 1)
        return f"{scheme}://***@{rest.split('@', 1)[1]}"
    return value
