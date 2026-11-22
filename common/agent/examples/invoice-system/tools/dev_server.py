#!/usr/bin/env python3
"""Local dev server: the invoices function on an in-memory store.

    python3 tools/dev_server.py
    curl -s localhost:8099/health

This skips NATS so you can poke at the API. The deployed function uses
JetStream KV through `core/runtime.py`.
"""

from __future__ import annotations

import asyncio
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1] / "functions" / "python"))

from invoices.core.http import HttpApp, Request  # noqa: E402
from invoices.core.service import InvoiceService  # noqa: E402
from invoices.core.store import MemoryStore  # noqa: E402

PORT = 8099
app = HttpApp(InvoiceService(MemoryStore()))


class Handler(BaseHTTPRequestHandler):
    server_version = "invoices-dev/0.1"

    def _dispatch(self) -> None:
        parsed = urlparse(self.path)
        length = int(self.headers.get("content-length") or 0)
        body = self.rfile.read(length) if length else b""
        request = Request(
            method=self.command,
            path=parsed.path,
            query={key: values[-1] for key, values in parse_qs(parsed.query).items()},
            headers={key.lower(): value for key, value in self.headers.items()},
            body=body,
        )
        envelope = asyncio.run(app.handle(request))
        payload = json.dumps(envelope).encode("utf-8")
        if envelope.get("ok"):
            status = envelope.get("status", 200)
        else:
            status = envelope.get("status", 500)
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    do_GET = _dispatch
    do_POST = _dispatch
    do_PATCH = _dispatch
    do_DELETE = _dispatch

    def log_message(self, fmt: str, *args) -> None:  # noqa: A003
        sys.stderr.write("%s %s\n" % (self.command, self.path))


if __name__ == "__main__":
    print(f"invoices dev server listening on http://localhost:{PORT}", file=sys.stderr)
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
