# Database Schema v1

## Purpose

This document defines the initial MariaDB schema direction for PriceAlert / DealHunt v1.

The schema is derived from the domain model and runtime design already agreed on.
It is intended to support:

- single-user local operation
- periodic scanning
- grouped market snapshots
- alert history
- future JSON-ready application contracts

This document is still a design document, not final SQL.
Its purpose is to lock structure and responsibilities before implementation.

---

## Design Principles

### 1. Domain-driven, not table-driven
The schema should support the domain model.
It must not redefine the product logic.

### 2. Keep raw and grouped data separate
Raw marketplace observations and grouped user-facing results must live separately.

### 3. Favor clarity over premature optimization
v1 should be easy to reason about first.
Optimization can follow once real usage patterns appear.

### 4. Persist meaningful state
Important runtime outcomes should be persisted, not only held in memory.

### 5. Plan retention early
Raw scraped data can grow quickly, so the schema should make pruning and retention manageable.

---

## ID Strategy Recommendation

### Recommended v1 choice
Use **string IDs in the application layer** and store them as `VARCHAR` in MariaDB.

A good practical format is:
- ULID
- or UUID

### Why
This keeps the design consistent with the documented domain model and JSON contracts.
It is also future-friendly for:
- replication of records across modes
- headless/API evolution
- avoiding tight coupling to auto-increment assumptions

### Recommendation
Use a fixed `VARCHAR` length appropriate for the chosen ID format.

Example:
- ULID: `CHAR(26)`
- UUID string: `CHAR(36)`

ULID is a strong fit because it is sortable and readable enough.

---

## Recommended Core Tables

For v1, the schema should include these primary tables:

1. `tracked_keywords`
2. `scan_jobs`
3. `raw_listings`
4. `grouped_listings`
5. `market_snapshots`
6. `price_points`
7. `alert_rules`
8. `alert_events`

These map directly to the selected domain model Option B.

---

## 1. tracked_keywords

### Purpose
Stores user-defined tracked search configurations.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| keyword | VARCHAR(255) | no | user-defined keyword |
| basic_filter | VARCHAR(100) | yes | optional lightweight filter |
| threshold_price | BIGINT | yes | optional threshold |
| interval_minutes | INT | no | scan interval |
| telegram_enabled | BOOLEAN | no | whether Telegram is enabled |
| status | VARCHAR(20) | no | `active`, `paused`, `archived` |
| created_at | DATETIME | no | creation time |
| updated_at | DATETIME | no | update time |

### Recommended indexes
- primary key on `id`
- index on `status`
- optional index on `updated_at`

### Notes
- single-user v1 does not require a `user_id` column.
- if multi-user is added later, this table will likely become tenant-scoped.

---

## 2. scan_jobs

### Purpose
Stores each monitoring execution for a tracked keyword.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| tracked_keyword_id | CHAR(26) | no | FK to tracked_keywords |
| started_at | DATETIME | no | scan start |
| finished_at | DATETIME | yes | scan end |
| status | VARCHAR(20) | no | `running`, `success`, `failed` |
| error_message | TEXT | yes | failure reason |
| raw_count | INT | yes | count of raw listings |
| grouped_count | INT | yes | count of grouped listings |

### Recommended indexes
- primary key on `id`
- index on `tracked_keyword_id`
- composite index on `(tracked_keyword_id, started_at)`
- optional index on `status`

### Foreign key
- `tracked_keyword_id` -> `tracked_keywords.id`

### Notes
- this table is important for observability and restart recovery.
- abandoned `running` jobs can be reconciled on startup.

---

## 3. raw_listings

### Purpose
Stores raw-ish marketplace listing observations from a scan.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| scan_job_id | CHAR(26) | no | FK to scan_jobs |
| source | VARCHAR(50) | no | e.g. `tokopedia` |
| title | TEXT | no | raw title |
| normalized_title | TEXT | no | normalized title used for grouping |
| seller_name | VARCHAR(255) | no | observed seller name |
| price | BIGINT | no | current observed price |
| original_price | BIGINT | yes | reference price if available |
| is_promo | BOOLEAN | no | whether observed as promo |
| url | TEXT | no | listing URL |
| scraped_at | DATETIME | no | scrape timestamp |

### Recommended indexes
- primary key on `id`
- index on `scan_job_id`
- optional index on `scraped_at`

### Foreign key
- `scan_job_id` -> `scan_jobs.id`

### Notes
- `normalized_title` should be persisted because it is useful for debugging grouping behavior.
- full-text indexing can be considered later, not required for v1.

---

## 4. grouped_listings

### Purpose
Stores grouped / deduplicated listing outputs for a given scan.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| scan_job_id | CHAR(26) | no | FK to scan_jobs |
| group_key | VARCHAR(255) | no | deterministic grouping identity |
| representative_title | TEXT | no | chosen display title |
| representative_seller | VARCHAR(255) | no | chosen seller |
| best_price | BIGINT | no | lowest price in group |
| original_price | BIGINT | yes | reference/original price if any |
| is_promo | BOOLEAN | no | promo state for representative |
| listing_count | INT | no | number of raw listings in group |
| sample_url | TEXT | no | chosen representative URL |

### Recommended indexes
- primary key on `id`
- index on `scan_job_id`
- composite index on `(scan_job_id, best_price)`
- optional index on `group_key`

### Foreign key
- `scan_job_id` -> `scan_jobs.id`

### Notes
- `group_key` does not need to be globally unique.
- it is scoped meaningfully by scan context.

---

## 5. market_snapshots

### Purpose
Stores the aggregated grouped market state for a tracked keyword at one point in time.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| tracked_keyword_id | CHAR(26) | no | FK to tracked_keywords |
| scan_job_id | CHAR(26) | no | FK to scan_jobs |
| min_price | BIGINT | yes | grouped min price |
| avg_price | BIGINT | yes | grouped average price |
| max_price | BIGINT | yes | grouped max price |
| raw_count | INT | no | count of raw listings |
| grouped_count | INT | no | count of grouped listings |
| signal | VARCHAR(20) | no | `BUY_NOW`, `GOOD_DEAL`, `NORMAL`, `NO_DATA` |
| snapshot_at | DATETIME | no | snapshot timestamp |

### Recommended indexes
- primary key on `id`
- index on `tracked_keyword_id`
- composite index on `(tracked_keyword_id, snapshot_at)`
- unique index on `scan_job_id` if one snapshot per scan is guaranteed

### Foreign keys
- `tracked_keyword_id` -> `tracked_keywords.id`
- `scan_job_id` -> `scan_jobs.id`

### Notes
- this is one of the most important tables for dashboard rendering.
- snapshot values should be derived from grouped listings, not raw listings.

---

## 6. price_points

### Purpose
Stores historical summary points for trend/history use.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| tracked_keyword_id | CHAR(26) | no | FK to tracked_keywords |
| scan_job_id | CHAR(26) | no | FK to scan_jobs |
| min_price | BIGINT | yes | historical min |
| avg_price | BIGINT | yes | historical avg |
| max_price | BIGINT | yes | historical max |
| recorded_at | DATETIME | no | history point time |

### Recommended indexes
- primary key on `id`
- index on `tracked_keyword_id`
- composite index on `(tracked_keyword_id, recorded_at)`

### Foreign keys
- `tracked_keyword_id` -> `tracked_keywords.id`
- `scan_job_id` -> `scan_jobs.id`

### Notes
- this can mirror snapshot summary values in v1.
- keeping a separate table still helps if retention policies differ later.

---

## 7. alert_rules

### Purpose
Stores alert configuration rules.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| tracked_keyword_id | CHAR(26) | no | FK to tracked_keywords |
| rule_type | VARCHAR(50) | no | e.g. `threshold_below`, `new_lowest` |
| value | TEXT | no | rule parameter payload |
| enabled | BOOLEAN | no | active flag |
| created_at | DATETIME | no | creation time |

### Recommended indexes
- primary key on `id`
- index on `tracked_keyword_id`
- optional composite index on `(tracked_keyword_id, rule_type, enabled)`

### Foreign key
- `tracked_keyword_id` -> `tracked_keywords.id`

### Notes
- this table can remain lightly used in early implementation.
- `value` may later evolve to JSON content if needed.

---

## 8. alert_events

### Purpose
Stores user-facing and operational alert/log events.

### Suggested columns

| Column | Type | Null | Notes |
|---|---|---:|---|
| id | CHAR(26) | no | ULID recommended |
| tracked_keyword_id | CHAR(26) | no | FK to tracked_keywords |
| scan_job_id | CHAR(26) | yes | optional FK to scan_jobs |
| level | VARCHAR(20) | no | `INFO`, `ALERT`, `WARN`, `ERROR` |
| event_type | VARCHAR(50) | no | semantic event type |
| message | TEXT | no | human-readable message |
| payload_json | LONGTEXT | yes | optional structured payload |
| sent_to_telegram | BOOLEAN | no | notifier delivery semantic chosen by app |
| created_at | DATETIME | no | event timestamp |

### Recommended indexes
- primary key on `id`
- index on `tracked_keyword_id`
- index on `scan_job_id`
- composite index on `(tracked_keyword_id, created_at)`
- optional composite index on `(tracked_keyword_id, event_type, created_at)`

### Foreign keys
- `tracked_keyword_id` -> `tracked_keywords.id`
- `scan_job_id` -> `scan_jobs.id`

### Notes
- this table powers the activity log and alert-history lookups.
- if delivery semantics become richer later, this table may expand.

---

## Suggested Relationship Summary

```text
tracked_keywords
  ├── scan_jobs
  │     ├── raw_listings
  │     ├── grouped_listings
  │     └── market_snapshots
  │
  ├── price_points
  ├── alert_rules
  └── alert_events
```

### Relationship notes
- one tracked keyword -> many scan jobs
- one scan job -> many raw listings
- one scan job -> many grouped listings
- one scan job -> one market snapshot
- one tracked keyword -> many price points
- one tracked keyword -> many alert rules
- one tracked keyword -> many alert events

---

## Suggested Retention Strategy

Raw data can grow quickly, so retention policy should be planned early.

### Recommended v1 retention approach

#### raw_listings
- keep for short or medium retention window
- example: 7 to 30 days depending on local usage

#### grouped_listings
- keep longer than raw_listings if useful
- example: 30 to 90 days

#### market_snapshots
- keep longer, these are lightweight and important

#### price_points
- keep long enough for trend/history

#### alert_events
- keep longer because they support user trust and debugging

### Recommendation
Retention can be implemented later, but the schema should assume that pruning jobs will exist.

---

## Optional Future Tables (Not Required in v1)

These are intentionally excluded from the v1 schema, but may appear later:

- `notifier_delivery_attempts`
- `runtime_locks`
- `app_settings`
- `source_credentials`
- `scrape_sessions`
- `grouping_debug_records`

Do not create these yet unless implementation clearly needs them.

---

## Constraints and Validation Recommendations

### Suggested application-level validation
- `interval_minutes` must be > 0
- `threshold_price` if present must be > 0
- `status` must be valid enum value
- `signal` must be valid enum value
- `level` must be valid enum value
- `rule_type` must be valid supported value

### Suggested DB-level discipline
- NOT NULL for fields that are always required
- foreign keys for clear ownership links
- indexes for common query paths

### Important note
Do not push every validation rule into SQL constraints only.
Domain/application layer should still validate meaning.

---

## Query Patterns to Optimize For

The schema should mainly support these read paths:

### Dashboard
- list tracked keywords
- fetch latest snapshot for selected keyword
- fetch recent alert events
- fetch grouped listings for latest scan

### Detail view
- fetch tracked keyword
- fetch latest snapshot
- fetch grouped listings for latest scan
- fetch recent price_points
- fetch recent alert_events

### Worker/runtime
- create scan_job
- persist raw_listings
- persist grouped_listings
- persist market_snapshot
- persist price_point
- persist alert_events
- mark scan_job success/failure

### Alert engine
- fetch recent alert_events for same keyword/type
- fetch recent price_points / snapshots

---

## Potential Schema Risks

### 1. Raw data growth
The biggest likely storage risk.

#### Mitigation
Plan retention and pruning early.

### 2. Over-indexing too early
Too many indexes can slow writes.

#### Mitigation
Start with essential indexes only.

### 3. Ambiguous `sent_to_telegram`
This field can become semantically fuzzy.

#### Mitigation
Clarify in app logic whether it means:
- attempted
- succeeded
- accepted by API

If needed later, evolve to richer delivery tracking.

### 4. Snapshot duplication errors
If code accidentally writes multiple snapshots per scan, dashboard logic becomes messy.

#### Mitigation
Prefer one snapshot per scan and enforce this with application logic or unique index.

### 5. Retention mismatch
If `price_points` and `market_snapshots` diverge without a clear reason, logic may become confusing.

#### Mitigation
Document why both exist and keep their relationship simple in v1.

---

## Recommended v1 Simplification Option

If implementation pressure is high, one simplification is possible:

- keep the full schema design in docs
- but allow `price_points` to be written directly from `market_snapshots`
- and allow `alert_rules` to be minimally populated or partially implicit at first

This preserves future direction without overcomplicating the first milestone.

---

## Suggested Migration Order

When implementation begins, a sensible migration order is:

1. `tracked_keywords`
2. `scan_jobs`
3. `raw_listings`
4. `grouped_listings`
5. `market_snapshots`
6. `price_points`
7. `alert_rules`
8. `alert_events`

This order follows ownership dependencies cleanly.

---

## Recommended Next Documents

After database schema, the next most useful documents are:

1. `docs/project-structure-v1.md`
2. `docs/scraping-feasibility-notes.md`
3. `docs/implementation-plan-v1.md`

---

## Final Summary

The v1 database schema should remain:
- domain-aligned
- clear to query
- safe for single-user local runtime
- tolerant of future web/API expansion

The most important design choices are:
- separate raw and grouped data
- treat snapshots and events as first-class persisted outputs
- plan retention for raw data early
- keep indexes focused on dashboard, detail, worker, and alert query paths

If those are preserved, the schema will support both a clean POC and a reasonable growth path.
