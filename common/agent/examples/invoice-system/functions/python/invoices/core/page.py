"""A transport-neutral page/asset response.

API routes return dictionaries that the runtime serialises as JSON. UI routes
return a `Page` instead: HTML, CSS or JS with its own status and headers. The
sync runtime turns a `Page` into a Starlette response, and the local dev server
writes it straight to the socket, so both environments serve the same bytes.
"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class Page:
    body: str | bytes
    status: int = 200
    content_type: str = "text/html; charset=utf-8"
    headers: dict[str, str] = field(default_factory=dict)

    def as_bytes(self) -> bytes:
        if isinstance(self.body, bytes):
            return self.body
        return self.body.encode("utf-8")

    @property
    def location(self) -> str | None:
        return self.headers.get("Location")


def html(body: str, *, status: int = 200) -> Page:
    return Page(body=body, status=status, content_type="text/html; charset=utf-8")


def redirect(location: str, *, status: int = 303) -> Page:
    return Page(body=b"", status=status, headers={"Location": location})
