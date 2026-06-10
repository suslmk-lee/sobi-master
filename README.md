# sobi-master

**English** | [한국어](README.ko.md)

A family budget desktop app built with Go + Wails v2 + React/TS + Supabase (Postgres).

Import card/bank CSV statements or register transactions manually. Once you label a
transaction, a "merchant + amount range" fingerprint rule classifies the same recurring
charge automatically from the next month on. For example, three LG U+ auto-payments that
differ only in amount can be labeled once as Dad / Kid / Internet — after that, the app
tells you whose bill each charge is, every month, automatically.

## Features

- **Dashboard** — monthly income/expense/transfer summary with month-over-month deltas,
  6-month trend chart, daily spending + cumulative line, category/member donut charts,
  top merchants, per-card performance widgets
- **Transactions** — manual entry (cash, dues, gifts), month filter, unclassified-only
  view, edit via modal
- **Cards** — register cards with issuer, billing day, performance period and spending
  target; track whether the target is met in the current period; per-card breakdown by
  merchant and category; custom chip colors
- **Import** — CSV statements from Korean card companies/banks, automatic column
  detection (date/merchant/amount/deposit/withdrawal keywords), EUC-KR encoding support,
  duplicate skipping
- **Auto-classification** — fingerprint rules (merchant + amount ±8%) learned every time
  you classify a transaction; manage/delete rules in Settings

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

- `app.go` — Wails bindings (API called from the frontend, reconnect/logging)
- `internal/store` — Supabase (Postgres) schema/queries, config/log paths, SQLite migration
- `internal/classifier` — auto-classification rule learning/matching (±8% amount tolerance)
- `internal/importer` — card/bank CSV parser (header keyword detection, EUC-KR support)
- `frontend/src/pages` — Dashboard / Transactions (incl. manual entry) / Cards / Import / Settings

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
- **Rule**: learned automatically whenever you confirm a classification;
  review/delete in the Settings tab
