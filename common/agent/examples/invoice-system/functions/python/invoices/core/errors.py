"""Error types shared by the invoice domain and the HTTP/op adapters.

The LiteFunctions python runtime always answers HTTP 200 for sync calls, so an
error also carries the status code we *would* have returned. Callers should read
`status` from the error envelope.
"""

from __future__ import annotations

from typing import Any


class ApiError(Exception):
    status = 400
    code = "invalid_request"

    def __init__(
        self,
        message: str,
        *,
        code: str | None = None,
        status: int | None = None,
        details: Any = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        if code is not None:
            self.code = code
        if status is not None:
            self.status = status
        self.details = details

    def to_dict(self) -> dict[str, Any]:
        error: dict[str, Any] = {"code": self.code, "message": self.message}
        if self.details is not None:
            error["details"] = self.details
        return error


class ValidationError(ApiError):
    status = 422
    code = "validation_error"


class NotFoundError(ApiError):
    status = 404
    code = "not_found"


class ConflictError(ApiError):
    status = 409
    code = "conflict"


class StoreUnavailableError(ApiError):
    status = 503
    code = "store_unavailable"


class KeyExistsError(Exception):
    """Raised by a KV store when a create hits an existing key."""


class RevisionMismatchError(Exception):
    """Raised by a KV store when a compare-and-swap update loses the race."""
