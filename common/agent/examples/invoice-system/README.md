# Invoice system

A complete little invoicing service built as a **LiteFunctions python function
package**, with **NATS JetStream KV as the only datastore**.

One deployable function (`invoices`) exposes ~20 operations over three
transports:

- sync HTTP (`handle.py` receives the Starlette request),
- async pub/sub ops (`{"op": "...", ...}` payloads),
- and a local dev server on an in-memory store, for working without a cluster.

There is no Postgres, no Redis, no ORM. Customers, invoices, payments, invoice
number sequences and idempotency records all live in JetStream KV buckets.

```
                  ┌──────────────────────────┐
  HTTP / async ──▶│  handle.py               │
                  │    ▼                     │
                  │  core/http.py  (routes)  │
                  │    ▼                     │
                  │  core/service.py         │
                  │    ▼            ▼        │
                  │  models.py    events ────┼──▶ <project>.invoices.events.*
                  │    ▼                     │
                  │  Store (NATS KV / memory)│
                  └──────────────────────────┘
```

## Layout

```
common/agent/examples/invoice-system/
├── functions/python/invoices/     # deployable function package
│   ├── handle.py                  # sync + async entrypoint
│   └── core/
│       ├── errors.py              # ApiError with HTTP status
│       ├── money.py               # exact decimal <-> minor units
│       ├── models.py              # validation, totals, JSON presentation
│       ├── store.py               # KV abstraction, NATS JetStream KV, MemoryStore
│       ├── service.py             # all business operations
│       ├── http.py                # HTTP routes + async op mapping
│       └── runtime.py             # NATS connection / service singleton
├── tests/                         # stdlib unittest, no extra deps
├── tools/dev_server.py            # local server on MemoryStore
├── deploy/function.yaml           # Function CR
└── Makefile                       # test / dev / fmt
```

## Features

- Customers: create, get, list/search, update, archive/restore.
- Invoices: draft creation, edit drafts, issue (sequential numbering),
  send, void, delete drafts.
- Line items with quantity, unit price, per-line tax and discount.
- Exact money math: decimal strings on the wire, integer minor units in
  storage, no binary floats (JPY/KWD-style exponents supported).
- Payments: partial payments, status transitions (`open → partially_paid →
  paid`), void a payment to reopen the invoice, overpayment rejected.
- Idempotent creates for customers, invoices and payments via
  `Idempotency-Key` (stored in KV, fingerprint-checked).
- Derived `is_overdue` / `days_overdue`, filters by status, customer, text and
  overdue.
- Summary report per currency: invoiced, paid, outstanding, overdue, drafts.
- HTML invoice rendering (escaped) — ready to feed a PDF renderer later.
- Domain events on NATS: `customer.created`, `invoice.issued`,
  `payment.recorded`, `invoice.paid`, ...
- Compare-and-swap writes so concurrent updates cannot silently overwrite
  each other.

## Run locally

```bash
cd common/agent/examples/invoice-system
make test          # 26 tests, stdlib only
make dev           # http://localhost:8099, in-memory store
make clean         # drop __pycache__ / ruff cache
PROJECT=one make local-push   # push package + apply CR + wait for runtime
```

`local-push` expects a reachable local cluster with the LiteFunctions Gitea and
the `litefunctions-admin-token` secret. Override `NAMESPACE`, `GITEA_URL`,
`GIT_OWNER`, `GIT_TOKEN` or `INGESTOR_URL` if your install differs.

Smoke test the dev server:

```bash
curl -s localhost:8099/health

CUSTOMER=$(curl -s localhost:8099/customers -H 'content-type: application/json' \
  -d '{"name":"Acme Corp","email":"billing@acme.test","currency":"USD","payment_terms_days":14}' \
  | jq -r .data.id)

INVOICE=$(curl -s localhost:8099/invoices -H 'content-type: application/json' \
  -d "{\"customer_id\":\"$CUSTOMER\",\"line_items\":[
        {\"description\":\"Consulting\",\"quantity\":2,\"unit_price\":\"100.00\",\"tax_rate_bp\":825},
        {\"description\":\"Design\",\"quantity\":1.5,\"unit_price\":\"80.00\",\"discount_bp\":1000}]}" \
  | jq -r .data.id)

curl -s -X POST localhost:8099/invoices/$INVOICE/issue | jq
curl -s localhost:8099/invoices/$INVOICE/payments -H 'content-type: application/json' \
  -d '{"amount":"100.00","method":"bank_transfer","reference":"TX-1"}' | jq
curl -s localhost:8099/summary | jq
```

Note the dev server returns real HTTP status codes. The **deployed** runtime
always returns HTTP 200 for sync calls, so read `status` from the envelope
(below).

## Deploy on LiteFunctions

1. Copy `functions/python/invoices/` into the git repo of a LiteFunctions
   project (the layout is `functions/python/<function>/handle.py`), commit and
   push. The python runtime clones that repo and hot-reloads on hook events.
2. Apply the Function CR, replacing `REPLACE_ME` with the project name:

   ```bash
   sed 's/REPLACE_ME/<project>/' deploy/function.yaml | kubectl apply -f -
   ```

3. Create/activate the endpoint in the Portal (or via the existing endpoint
   API). The ingestor proxies sync calls to the runtime with
   `X-Litefunction-Name: invoices`.
4. The ingestor exposes exactly `/lambda/<project>/<name>` and forwards the
   runtime path as `/`, so use the op interface on the deployed endpoint
   (REST subpaths below work for direct runtime calls and the dev server):

   ```bash
   export BASE=http://localhost:32130/lambda/<project>/invoices

   curl -s $BASE | jq                                  # health
   curl -s "$BASE?op=list_invoices&status=open" | jq
   curl -s "$BASE?op=create_customer" -H 'content-type: application/json' \
     -d '{"name":"Acme Corp","currency":"USD"}' | jq
   curl -s "$BASE?op=create_invoice" -H 'content-type: application/json' \
     -d '{"customer_id":"cus_...","line_items":[{"description":"Work","quantity":1,"unit_price":"100.00"}]}' | jq
   curl -s "$BASE?op=issue_invoice" -H 'content-type: application/json' \
     -d '{"id":"inv_..."}' | jq
   ```

   Ops also work as a JSON field (`{"op":"record_payment", ...}`), and every
   op accepts `{"input": {...}}` plus `query`/`headers`/`idempotency_key`.

Set `isAsync: true` on the Function CR instead to use the op interface without
an HTTP endpoint. The op names are the same ones used with `?op=`:

```json
{"op": "issue_invoice", "invoice_id": "inv_abc123"}
{"op": "record_payment", "invoice_id": "inv_abc123", "amount": "50.00",
 "idempotency_key": "payment-2026-03-01"}
```

## API

Every response uses the same envelope:

```json
{"ok": true,  "status": 201, "data": { ... }}
{"ok": false, "status": 422, "error": {"code": "validation_error", "message": "..."}}
```

The paths below are the direct runtime/dev-server routes. The deployed
ingestor only forwards `/lambda/<project>/<name>` (runtime path `/`), so use
`?op=<name>` or an `"op"` body field there — same operations, same envelope.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/health` | liveness + which store is wired |
| POST | `/customers` | create customer |
| GET | `/customers?status=&q=&limit=&offset=` | list/search customers |
| GET | `/customers/{id}` | get customer |
| PATCH | `/customers/{id}` | update customer |
| POST | `/customers/{id}/archive` / `/restore` | archive / restore |
| DELETE | `/customers/{id}` | alias for archive |
| POST | `/invoices` | create draft |
| GET | `/invoices?status=&customer_id=&q=&overdue=&limit=&offset=` | list invoices |
| GET | `/invoices/{id}` | get invoice |
| PATCH | `/invoices/{id}` | edit draft (line items, dates, notes) |
| DELETE | `/invoices/{id}` | delete draft |
| POST | `/invoices/{id}/issue` | assign number, `draft → open` |
| POST | `/invoices/{id}/send` | stamp `sent_at`, emit event |
| POST | `/invoices/{id}/void` | void an unpaid issued invoice |
| POST | `/invoices/{id}/payments` | record a payment |
| GET | `/invoices/{id}/payments` | list payments |
| POST | `/invoices/{id}/payments/{payment_id}/void` | reverse a payment |
| GET | `/invoices/{id}/render` | HTML invoice in `data.html` |
| GET | `/summary` | per-currency totals and status counts |

`Idempotency-Key` is honoured on `POST /customers`, `POST /invoices` and
`POST /invoices/{id}/payments`.

### Money

Amounts are **decimal strings in major units** (`"100.00"`, `"1.5"` quantity).
Internally everything is an integer of minor units. `tax_rate_bp` and
`discount_bp` are basis points (`825` = 8.25%). Rounding is half-up per line.
The API returns `totals` with `subtotal`, `discount`, `tax`, `total`,
`amount_paid` and `balance_due`.

### Invoice lifecycle

```
draft ──issue──▶ open ──payment──▶ partially_paid ──payment──▶ paid
  │               │                      │
  │               └──────void────────────┘   (only when nothing is paid)
  └──delete
```

## Storage (JetStream KV)

All buckets are named `<INVOICES_KV_PREFIX>_<bucket>`:

| Bucket | Keys | Contents |
| --- | --- | --- |
| `customers` | `cust.<id>` | customer documents |
| `invoices` | `inv.<id>` | invoice documents + line items + totals |
| `payments` | `pay.<invoice_id>.<payment_id>` | payments, scoped per invoice |
| `meta` | `seq.invoice.<year>` | yearly invoice number counters (CAS updates) |
| `meta` | `idem.<scope>.<key>` | idempotency fingerprints → entity ids |

The service uses JetStream KV `create` for inserts and revision-checked
`update` for compare-and-swap, so two concurrent payments cannot both spend the
same outstanding balance. Invoice numbers use the same CAS loop.

| Env var | Default | Purpose |
| --- | --- | --- |
| `NATS_URL` | (required, set by the platform) | JetStream endpoint |
| `PROJECT` | `default` | prefixes KV buckets and event subjects |
| `INVOICES_KV_PREFIX` | `<PROJECT>_invoices` | override bucket prefix |
| `INVOICES_EVENTS_SUBJECT` | `<PROJECT>.invoices.events` | event subject prefix |

## Design notes / limits

- The LiteFunctions sync runtime returns HTTP 200 regardless of handler
  outcome. The authoritative status is `status` inside the envelope; the dev
  server maps it to real HTTP status codes.
- Listing scans keys in a bucket (no secondary indexes yet). That is fine for
  thousands of invoices; add per-customer/per-status index keys before that.
- Invoice numbers can leave gaps if issuing fails after a number is allocated.
  Gaps are preferable to duplicates.
- Payments are stored before the invoice balance is updated; a crash in between
  can leave a payment that still needs reconciliation. A real deployment should
  run a reconciler over `pay.*` keys.
- Domain events are core NATS (fire-and-forget). Swap `EventPublisher` for a
  JetStream publisher if events must be durable.
- `render` returns HTML. Add a PDF renderer (e.g. WeasyPrint) as a separate
  function/worker if you need PDFs.
- Auth/tenancy come from the platform (project + endpoint layer); this package
  does not implement per-user access.
