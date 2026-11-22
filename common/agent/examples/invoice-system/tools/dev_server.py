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
import os
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1] / "functions" / "python"))

from invoices.core.http import HttpApp, Request  # noqa: E402
from invoices.core.page import Page  # noqa: E402
from invoices.core.service import InvoiceService  # noqa: E402
from invoices.core.store import MemoryStore  # noqa: E402

# Links in the UI are prefix-free when the dev server serves the package
# directly. The deployed runtime sets PROJECT and prefixes them with
# /lambda/<project>/invoices instead.
os.environ["INVOICES_UI_BASE"] = ""

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
        result = asyncio.run(app.handle(request))
        if isinstance(result, Page):
            payload = result.as_bytes()
            status = result.status
            content_type = result.content_type
            headers = result.headers
        else:
            envelope = result
            payload = json.dumps(envelope).encode("utf-8")
            status = envelope.get("status", 200) if envelope.get("ok") else envelope.get("status", 500)
            content_type = "application/json"
            headers = {}
        self.send_response(status)
        self.send_header("content-type", content_type)
        self.send_header("content-length", str(len(payload)))
        for key, value in headers.items():
            self.send_header(key, value)
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
