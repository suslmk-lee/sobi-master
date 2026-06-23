# sobi-master

**English** | [한국어](README.ko.md)

A family budget desktop app built with Go + Wails v2 + React/TS + Supabase (Postgres).

Import card/bank CSV statements or register transactions manually. Once you label a
transaction, a "merchant + amount range" fingerprint rule classifies the same recurring
charge automatically from the next month on. For example, three LG U+ auto-payments that
differ only in amount can be labeled once as Dad / Kid / Internet — after that, the app
tells you whose bill each charge is, every month, automatically.

## Features

- **Dashboard** — monthly income/expense/transfer summary with month-over-month deltas, an
  alert panel (budget overruns, card-target risk, unclassified backlog), 6-month trend chart,
  daily spending + cumulative line, per-category budget gauges, category/member donut charts,
  top merchants, per-card performance widgets
- **Transactions** — manual entry (cash, dues, gifts); search by merchant/memo plus
  amount/period/category/member/payment-method filters and sorting; edit via modal;
  multi-select bulk re-classify/delete with one-click undo; "apply rules to unclassified"
  to retroactively classify; CSV export of the current view (Excel-friendly, UTF-8 BOM)
- **Cards** — register cards with issuer, billing day, performance period and spending
  target; track whether the target is met in the current period; per-card breakdown by
  merchant and category; custom chip colors
- **Statistics** (dedicated tab) — daily / cumulative spending by card, member, or category
  (dimension + mode toggles); this-month vs. previous-month cumulative comparison at the same
  day; weekday spending averages; 6-month category trend; per-card category composition
  (stacked bars); per-member analysis (category composition, 6-month trend, and a summary
  card per member with total, share, top category/merchant and month-over-month change);
  card performance pace toward target; recurring/fixed-cost tracker (which monthly charges
  have or haven't posted yet); yearly summary with year-over-year deltas
- **Budget** — set a recurring monthly budget per category, with optional per-month overrides;
  the dashboard shows usage gauges and raises alerts on overruns
- **Import** — CSV statements from Korean card companies/banks, automatic column
  detection (date/merchant/amount/deposit/withdrawal keywords), EUC-KR encoding support,
  duplicate skipping
- **Auto-classification** — a fingerprint rule (merchant + amount ±8%) is learned whenever you
  classify a transaction. On manual entry and import a matching rule takes priority over form
  defaults and classifies automatically. Add/edit/delete rules manually in Settings
- **Dark mode** — toggle in the header, remembered across launches

## Running

```sh
wails dev      # development mode (hot reload)
wails build    # production build → build/bin/sobi.app (.exe when built on Windows)
```

## Configuration (config.json) — required

The app needs a Supabase connection string. On first launch an empty template file is
created — open it and fill in the value.

### Path

| OS      | Path                                                  |
|---------|-------------------------------------------------------|
| macOS   | `~/Library/Application Support/sobi/config.json`       |
| Windows | `%AppData%\sobi\config.json` (usually `C:\Users\<name>\AppData\Roaming\sobi\config.json`) |
| Linux   | `~/.config/sobi/config.json`                            |

### Content

```json
{
  "database_url": "postgresql://postgres.xxxxxxxxxxxx:PASSWORD@aws-1-ap-southeast-1.pooler.supabase.com:5432/postgres"
}
```

- Get the URI from the Supabase dashboard → **Connect** → **Session pooler**.
  (Direct connection is IPv6-only; Session pooler is recommended for home networks.
  Transaction pooler on port 6543 also works — the app switches to a compatible mode
  automatically.)
- Replace `[YOUR-PASSWORD]` with the actual DB password. Special characters such as
  `* : @ / !` can be pasted as-is — the app URL-encodes them automatically.
- Saving with Windows Notepad is fine (UTF-8 BOM is tolerated).

Put the same `database_url` into config.json on another PC to share the same ledger.

### Data migration

On connect, if Supabase is empty and a legacy local DB (`sobi.db`) exists in the same
folder, all data is migrated to Supabase automatically (the local file is kept as a backup).

## Log file (sobi.log)

All backend errors (connection failures, insert/update errors) are logged here.
Check this file first when something goes wrong — the app creates it automatically.

| OS      | Path                                              |
|---------|----------------------------------------------------|
| macOS   | `~/Library/Application Support/sobi/sobi.log`       |
| Windows | `%AppData%\sobi\sobi.log`                            |
| Linux   | `~/.config/sobi/sobi.log`                            |

If the connection fails the app does not quit; an error banner appears at the top and
the "reconnect" button recovers without a restart once the config is fixed.

## Project layout

- `app.go` — Wails bindings (API called from the frontend, reconnect/logging, aggregate
  dashboard endpoint, CSV export, alerts, recurring/budget helpers)
- `internal/store` — Supabase (Postgres) schema/queries; analytics & statistics (`analytics.go`,
  `stats.go`); budgets (`budget.go`); config/log paths; SQLite migration
- `internal/classifier` — auto-classification rule learning/matching (±8% amount tolerance)
- `internal/importer` — card/bank CSV parser (header keyword detection, EUC-KR support)
- `frontend/src/pages` — Dashboard / Transactions (incl. manual entry) / Cards / Statistics / Import / Settings

## Tests

DB tests need Postgres (skipped automatically when unavailable):

```sh
docker run -d --rm --name sobi-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=sobi -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/sobi?sslmode=disable' go test ./...
docker stop sobi-test-pg
```

## Concepts

- **Member**: who the money was spent for (Dad / Mom / Kid / Shared)
- **Category**: purpose; `kind` distinguishes income/expense/transfer
  (salary, telecom, loan repayment, dues, investment transfer, …)
- **Payment method**: which card/cash/bank account it went through; cards carry
  billing day / performance period / target managed in the Cards tab
- **Budget**: an optional monthly spending limit per category; set a recurring default and
  override specific months in Settings; the dashboard tracks usage and flags overruns
- **Rule**: learned automatically whenever you confirm a classification; on a match it takes
  priority over manual-entry defaults; review/add/edit/delete in the Settings tab. A rule that
  recurs across months also drives the Statistics tab's fixed-cost tracker
