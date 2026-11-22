"""LiteFunctions python entrypoint.

The same package works for:
  * sync HTTP calls (the runtime passes a Starlette Request)
  * async pub/sub calls (the runtime passes a payload wrapper)
"""

from __future__ import annotations

import logging

from .core.errors import ApiError
from .core.http import HttpApp
from .core.runtime import error_envelope, get_service

logger = logging.getLogger(__name__)


async def handle(request):
    try:
        service = await get_service()
    except ApiError as err:
        return error_envelope(err)
    except Exception:
        logger.exception("failed to initialise the invoice service")
        return {
            "ok": False,
            "status": 500,
            "error": {
                "code": "service_init_failed",
                "message": "invoice service could not be initialised",
            },
        }

    app = HttpApp(service)

    if _is_http_request(request):
        return await app.from_starlette(request)

    try:
        payload = await request.json()
    except Exception:
        return {
            "ok": False,
            "status": 422,
            "error": {
                "code": "validation_error",
                "message": "async payload must be valid JSON",
            },
        }
    return await app.from_op(payload)


def _is_http_request(request) -> bool:
    return hasattr(request, "method") and hasattr(request, "url")
