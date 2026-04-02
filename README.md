# PriceAlert

PriceAlert is a Go-based local-first marketplace monitoring tool focused on helping users detect strong buying opportunities earlier.

The initial direction of the project is:

- single-user first
- TUI-first experience
- Tokopedia-focused for early validation
- MariaDB as primary storage
- grouped market snapshots instead of raw noisy listing output
- Telegram as the first external alert channel
- future-ready for JSON/API and web interface evolution

## Product Direction

The product is not intended to be just a raw price tracker.
Its direction is closer to:

> a local decision-support tool for buying at a better time

In practice, the first versions focus on:
- keyword-based monitoring
- grouped / deduplicated listing results
- market snapshots
- threshold and new-lowest alerts
- keyboard-first workflow

## Current Status

This repository is no longer only in planning.

The current branch already includes a working v1 foundation for:
- MariaDB bootstrap, migrations, and repositories
- grouped listing pipeline
- snapshot and price history creation
- alert evaluation with anti-spam
- single-keyword scan orchestration
- single-process scheduler/worker runtime
- Tokopedia scraper feasibility adapter
- DTO/query layer
- Bubble Tea TUI dashboard/detail flow
- tracked keyword add/edit/pause/resume/archive flows
- Telegram notifier foundation
- startup reconciliation and bounded startup maintenance

What is still intentionally incomplete:
- scraper hardening beyond the current feasibility adapter
- broader operational polish and observability improvements
- future API/web interface work
- any multi-user or multi-marketplace support

The docs directory contains the source-of-truth design documents for:
- product scope
- domain model
- JSON contracts
- grouping strategy
- worker runtime
- alert strategy
- database schema
- project structure
- implementation sequence

## Key Design Decisions

### Runtime
- single local process for v1
- no separate daemon in v1
- scheduler + worker live inside the application runtime
- no overlapping scans for the same tracked keyword

### Data model
- raw listings and grouped listings are stored separately
- grouped listings drive market snapshots and alerts
- snapshots and alert events are first-class persisted outputs

### Alerting
- alerts are based on grouped market state, not raw listing noise
- conservative v1 focus: threshold hit + new lowest
- anti-spam behavior is required, not optional

### Architecture
- domain-first design
- JSON-ready internal contracts
- TUI is a consumer of DTO/query outputs, not raw DB rows
- future web/API support should not require a major redesign

## Planned v1 Scope

- Go project structure
- MariaDB migrations and repositories
- grouping engine
- market snapshot logic
- alert logic
- local scheduler/worker runtime
- TUI dashboard and detail views
- Telegram alert integration

Most of this scope is now implemented in foundation form on the active development branch.
The remaining work is primarily hardening, maintenance, and operational refinement.

## Running Locally

### Prerequisites

- Go installed locally
- MariaDB running locally or reachable from your machine
- a database created for this app, for example `pricealert`

### 1. Set environment variables

The app is environment-driven. A minimal local setup looks like:

```bash
export PRICEALERT_DB_HOST=127.0.0.1
export PRICEALERT_DB_PORT=3306
export PRICEALERT_DB_USER=root
export PRICEALERT_DB_PASSWORD=password
export PRICEALERT_DB_NAME=pricealert
```

Optional but commonly useful:

```bash
export PRICEALERT_MIN_SCAN_INTERVAL_MINS=5
export PRICEALERT_MAX_CONCURRENT_SCANS=1
export PRICEALERT_RAW_LISTING_RETENTION_HOURS=336
export PRICEALERT_ALERT_EVENT_RETENTION_HOURS=720
```

Telegram is optional. Only set both of these if you want outbound alerts enabled:

```bash
export PRICEALERT_TELEGRAM_BOT_TOKEN=...
export PRICEALERT_TELEGRAM_CHAT_ID=...
```

### 2. Apply the SQL migrations

At the moment the app expects the schema to already exist. Apply:

- [migrations/001_init.sql](/home/iwan/Project/pricealert/migrations/001_init.sql)
- [migrations/002_price_points.sql](/home/iwan/Project/pricealert/migrations/002_price_points.sql)

Example with MariaDB client:

```bash
mariadb -h127.0.0.1 -uroot -ppassword pricealert < migrations/001_init.sql
mariadb -h127.0.0.1 -uroot -ppassword pricealert < migrations/002_price_points.sql
```

Adjust host/user/password/database to match your local setup.

### 3. Run the app

From the repository root:

```bash
go run ./cmd/pricealert
```

This starts the single-process local app:
- Bubble Tea TUI
- bounded runtime scheduler/worker
- Tokopedia scraper feasibility adapter
- startup reconciliation and startup-bounded maintenance

### Notes

- if Telegram config is omitted, the app still runs and uses a no-op notifier
- startup maintenance may reconcile abandoned `running` scan jobs and prune old raw listings / alert events
- the Tokopedia adapter is still a feasibility adapter, so scraper behavior may need further hardening

## Recommended Build Order

The implementation plan intentionally prioritizes trust and correctness before UI polish.

High-level order:
1. project skeleton
2. database and persistence
3. domain types
4. grouping engine
5. snapshot/history logic
6. alert logic
7. scan orchestration
8. scraper feasibility adapter
9. runtime scheduler/worker
10. DTO/query layer
11. TUI
12. Telegram integration

See `docs/implementation-plan-v1.md` for the full phased plan.

## Docs Map

Recommended reading order:

1. `docs/plan-v1.md`
2. `docs/domain-model-v1.md`
3. `docs/json-contracts-v1.md`
4. `docs/review-gaps-risks-v1.md`
5. `docs/grouping-strategy-v1.md`
6. `docs/worker-runtime-v1.md`
7. `docs/alert-strategy-v1.md`
8. `docs/database-schema-v1.md`
9. `docs/project-structure-v1.md`
10. `docs/implementation-plan-v1.md`

## Notes for Implementation Assistants

If this repository is used with coding agents such as Codex, the files in `docs/` should be treated as the source of truth.

Implementation should follow the documented boundaries:
- keep domain clean
- keep scraper logic in infrastructure
- keep TUI separate from repositories
- prefer grouped-state correctness over premature UI complexity

## Early Priorities

The highest-risk areas to validate early are:
- Tokopedia scraping feasibility
- grouping quality
- snapshot correctness
- alert anti-spam behavior

These should be treated as more important than visual polish in the first implementation milestones.
