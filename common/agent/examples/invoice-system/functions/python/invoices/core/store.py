"""KV storage.

The domain only knows about asynchronous key/value buckets with create and
compare-and-swap semantics. NATS JetStream KV implements that in production;
`MemoryStore` does the same in tests and local runs.

Buckets used by the service:

    customers   cust.<customer_id>
    invoices    inv.<invoice_id>
    payments    pay.<invoice_id>.<payment_id>
    meta        seq.invoice.<year>, idem.<scope>.<key>
"""

from __future__ import annotations

import asyncio
import base64
from dataclasses import dataclass
from typing import Any

from .errors import (
    KeyExistsError,
    RevisionMismatchError,
    StoreUnavailableError,
)

CUSTOMERS = "customers"
INVOICES = "invoices"
PAYMENTS = "payments"
META = "meta"


@dataclass(frozen=True)
class Value:
    data: bytes
    revision: int


class Store:
    async def get(self, bucket: str, key: str) -> Value | None:  # pragma: no cover
        raise NotImplementedError

    async def create(self, bucket: str, key: str, data: bytes) -> Value:  # pragma: no cover
        raise NotImplementedError

    async def update(self, bucket: str, key: str, data: bytes, revision: int) -> Value:  # pragma: no cover
        raise NotImplementedError

    async def put(self, bucket: str, key: str, data: bytes) -> Value:  # pragma: no cover
        raise NotImplementedError

    async def delete(self, bucket: str, key: str) -> None:  # pragma: no cover
        raise NotImplementedError

    async def keys(self, bucket: str, prefix: str = "") -> list[str]:  # pragma: no cover
        raise NotImplementedError

    async def close(self) -> None:  # pragma: no cover
        return None


class MemoryStore(Store):
    """Deterministic in-memory store with the same CAS semantics as NATS KV."""

    def __init__(self) -> None:
        self._buckets: dict[str, dict[str, Value]] = {}
        self._revisions: dict[str, int] = {}

    def _bucket(self, bucket: str) -> dict[str, Value]:
        return self._buckets.setdefault(bucket, {})

    def _next(self, bucket: str) -> int:
        revision = self._revisions.get(bucket, 0) + 1
        self._revisions[bucket] = revision
        return revision

    async def get(self, bucket: str, key: str) -> Value | None:
        return self._bucket(bucket).get(key)

    async def create(self, bucket: str, key: str, data: bytes) -> Value:
        entries = self._bucket(bucket)
        if key in entries:
            raise KeyExistsError(key)
        value = Value(data=data, revision=self._next(bucket))
        entries[key] = value
        return value

    async def update(self, bucket: str, key: str, data: bytes, revision: int) -> Value:
        entries = self._bucket(bucket)
        current = entries.get(key)
        if current is None:
            raise RevisionMismatchError(f"{bucket}/{key} does not exist")
        if current.revision != revision:
            raise RevisionMismatchError(
                f"{bucket}/{key} revision is {current.revision}, expected {revision}"
            )
        value = Value(data=data, revision=self._next(bucket))
        entries[key] = value
        return value

    async def put(self, bucket: str, key: str, data: bytes) -> Value:
        value = Value(data=data, revision=self._next(bucket))
        self._bucket(bucket)[key] = value
        return value

    async def delete(self, bucket: str, key: str) -> None:
        self._bucket(bucket).pop(key, None)

    async def keys(self, bucket: str, prefix: str = "") -> list[str]:
        return sorted(key for key in self._bucket(bucket) if key.startswith(prefix))


class NatsStore(Store):
    """NATS JetStream KV backed store.

    nats-py is imported lazily so the domain and tests do not need the library.
    """

    def __init__(self, nc: Any, js: Any, prefix: str) -> None:
        self.nc = nc
        self.js = js
        self.prefix = _safe(prefix or "invoices")
        self._buckets: dict[str, Any] = {}
        self._lock = asyncio.Lock()

    @classmethod
    async def connect(cls, url: str, prefix: str = "invoices") -> "NatsStore":
        try:
            import nats
        except ImportError as exc:  # pragma: no cover - runtime image ships nats-py
            raise StoreUnavailableError(
                "nats-py is not installed in this runtime", code="store_dependency_missing"
            ) from exc

        try:
            nc = await nats.connect(
                url,
                name="litefunctions-invoices",
                max_reconnect_attempts=-1,
                reconnect_time_wait=1,
            )
            js = nc.jetstream()
        except Exception as exc:
            raise StoreUnavailableError(
                f"could not connect to NATS at {url}: {exc}", code="nats_connect_failed"
            ) from exc
        return cls(nc, js, prefix)

    def _bucket_name(self, bucket: str) -> str:
        return f"{self.prefix}_{_safe(bucket)}"

    async def _kv(self, bucket: str) -> Any:
        cached = self._buckets.get(bucket)
        if cached is not None:
            return cached

        async with self._lock:
            cached = self._buckets.get(bucket)
            if cached is not None:
                return cached

            from nats.js.api import KeyValueConfig
            from nats.js.errors import NotFoundError as JsNotFound

            name = self._bucket_name(bucket)
            try:
                kv = await self.js.key_value(name)
            except JsNotFound:
                kv = await self.js.create_key_value(
                    config=KeyValueConfig(
                        bucket=name,
                        history=1,
                        description="litefunctions invoice system",
                    )
                )
            self._buckets[bucket] = kv
            return kv

    async def get(self, bucket: str, key: str) -> Value | None:
        from nats.js.errors import KeyNotFoundError

        kv = await self._kv(bucket)
        try:
            entry = await kv.get(key)
        except KeyNotFoundError:
            return None
        if entry is None or entry.value is None:
            return None
        return Value(data=entry.value, revision=entry.revision)

    async def create(self, bucket: str, key: str, data: bytes) -> Value:
        from nats.js.errors import KeyWrongLastSequenceError

        kv = await self._kv(bucket)
        try:
            revision = await kv.create(key, data)
        except KeyWrongLastSequenceError as exc:
            raise KeyExistsError(key) from exc
        return Value(data=data, revision=revision)

    async def update(self, bucket: str, key: str, data: bytes, revision: int) -> Value:
        from nats.js.errors import KeyWrongLastSequenceError

        kv = await self._kv(bucket)
        try:
            revision = await kv.update(key, data, last=revision)
        except KeyWrongLastSequenceError as exc:
            raise RevisionMismatchError(str(exc)) from exc
        return Value(data=data, revision=revision)

    async def put(self, bucket: str, key: str, data: bytes) -> Value:
        kv = await self._kv(bucket)
        revision = await kv.put(key, data)
        return Value(data=data, revision=revision)

    async def delete(self, bucket: str, key: str) -> None:
        kv = await self._kv(bucket)
        try:
            await kv.delete(key)
        except Exception:
            # Delete is idempotent for our purposes; NATS raises for missing keys
            # on some versions.
            return

    async def keys(self, bucket: str, prefix: str = "") -> list[str]:
        from nats.js.errors import NoKeysError

        kv = await self._kv(bucket)
        try:
            keys = kv.keys()
            if asyncio.iscoroutine(keys):
                keys = await keys
        except NoKeysError:
            return []
        # The nats-py 2.10 client filters keys client-side, so ask for all of
        # them and apply the prefix ourselves. This stays correct across versions.
        return sorted(key for key in keys if key.startswith(prefix))

    async def close(self) -> None:
        try:
            await self.nc.drain()
        except Exception:
            try:
                await self.nc.close()
            except Exception:
                pass


def _safe(name: str) -> str:
    allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
    cleaned = "".join(ch if ch in allowed else "_" for ch in name)
    return cleaned[:100] or "invoices"


def encode_key_part(value: str) -> str:
    """Keys are subject tokens, so keep user supplied idempotency keys opaque."""
    raw = value.encode("utf-8")
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")[:120]
