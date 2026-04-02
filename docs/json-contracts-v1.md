# JSON Contracts v1

## Purpose

This document defines the initial JSON contract shapes for PriceAlert / DealHunt v1.

These contracts are intended to:

- support the TUI without coupling the UI directly to database tables
- support worker, alert, and notifier integration
- prepare the system for a future web interface and API layer
- keep a stable and understandable data boundary across components

The TUI remains the main user experience in v1, but the application should be JSON-ready from the start.

---

## Principles

### 1. JSON is a contract, not the product UI
JSON exists for:
- inter-component communication
- clean DTOs for rendering
- debugging and automation
- future API/web support

JSON is **not** the main user-facing experience for v1.

### 2. Contracts should be derived from domain models
We do not design JSON from database tables directly.
We derive JSON from domain concepts such as:
- tracked keyword
- snapshot
- grouped listing
- event log
- history

### 3. One shape per use case
Do not create one giant JSON object for everything.
Instead, use separate shapes for:
- dashboard
- detail view
- event log
- grouped listing output
- worker result

### 4. Keep names stable and explicit
Use explicit field names such as:
- `tracked_keyword_id`
- `snapshot_at`
- `grouped_count`

Avoid vague names such as:
- `data`
- `items`
- `value`

unless the nesting is already very clear.

### 5. Prefer machine-safe enums
Examples:
- `BUY_NOW`
- `GOOD_DEAL`
- `NORMAL`
- `NO_DATA`

instead of free-form human labels.

---

## Conventions

### Timestamps
Use ISO 8601 with timezone offset.

Example:
```json
"2026-04-02T10:40:00+07:00"
```

### Price fields
Use integer numeric values.

Example:
```json
23800
```

Do not store formatted display strings such as:
```json
"23.800"
```

Formatting belongs to the renderer/UI layer.

### Nullable values
Use `null` when data is intentionally missing.

Example:
- `original_price: null`
- `min_price: null`

### Boolean fields
Use explicit booleans, such as:
- `is_promo`
- `telegram_enabled`
- `has_new_alert`

---

## Contract List

Primary JSON contracts for v1:

1. TrackedKeyword JSON
2. TrackedKeywordSummary JSON
3. GroupedListing JSON
4. MarketSnapshot JSON
5. AlertEvent JSON
6. PricePoint JSON
7. KeywordDetail JSON
8. DashboardState JSON
9. RuntimeStatusSummary JSON
10. WorkerScanResult JSON
11. TelegramAlertPayload JSON

---

## 1. TrackedKeyword JSON

### Purpose
Used for tracking configuration views, keyword lists, and detail headers.

### Shape

```json
{
  "id": "kw_001",
  "keyword": "minyak goreng 2L",
  "basic_filter": "2L",
  "threshold_price": 25000,
  "interval_minutes": 5,
  "telegram_enabled": true,
  "status": "active",
  "created_at": "2026-04-02T10:00:00+07:00",
  "updated_at": "2026-04-02T10:10:00+07:00"
}
```

### Notes
- `threshold_price` may be `null`.
- `basic_filter` may be `null`.
- `status` should match domain enum values exactly.

---

## 2. TrackedKeywordSummary JSON

### Purpose
Used for lightweight keyword lists where full configuration detail is unnecessary.

### Shape

```json
{
  "id": "kw_001",
  "keyword": "minyak goreng 2L",
  "status": "active",
  "has_new_alert": true
}
```

### Notes
- this is the summary shape used by the dashboard keyword list.
- it is intentionally lighter than full `TrackedKeyword JSON`.

---

## 3. GroupedListing JSON

### Purpose
Used for top deals, detail views, and grouped result output.

### Shape

```json
{
  "id": "grp_001",
  "group_key": "minyak-goreng-2l-a",
  "representative_title": "Minyak Goreng 2L Promo",
  "representative_seller": "Seller A",
  "best_price": 23800,
  "original_price": 32000,
  "is_promo": true,
  "listing_count": 5,
  "sample_url": "https://example.com/item/123"
}
```

### Notes
- `original_price` may be `null`.
- `group_key` is primarily system-facing, but can remain in the payload for debugging or API use.

---

## 4. MarketSnapshot JSON

### Purpose
Used as the main summary object for one tracked keyword at one point in time.

### Shape

```json
{
  "id": "snap_001",
  "tracked_keyword_id": "kw_001",
  "scan_job_id": "scan_001",
  "min_price": 23800,
  "avg_price": 27500,
  "max_price": 34000,
  "raw_count": 20,
  "grouped_count": 8,
  "signal": "BUY_NOW",
  "snapshot_at": "2026-04-02T10:40:00+07:00"
}
```

### Notes
- `min_price`, `avg_price`, and `max_price` may be `null` when no valid data exists.
- `signal` should always be present.

---

## 5. AlertEvent JSON

### Purpose
Used for activity log, notifications, event history, and alert delivery status.

### Shape

```json
{
  "id": "evt_001",
  "tracked_keyword_id": "kw_001",
  "scan_job_id": "scan_001",
  "level": "ALERT",
  "event_type": "new_lowest",
  "message": "minyak goreng 2L hit new low: 23.800",
  "payload_json": null,
  "sent_to_telegram": true,
  "created_at": "2026-04-02T10:41:00+07:00"
}
```

### Notes
- `scan_job_id` may be `null` for system-level events.
- `payload_json` may be `null`.
- `message` should already be human-readable.

---

## 6. PricePoint JSON

### Purpose
Used for recent history and trend displays.

### Shape

```json
{
  "id": "pp_001",
  "tracked_keyword_id": "kw_001",
  "scan_job_id": "scan_001",
  "min_price": 23800,
  "avg_price": 27500,
  "max_price": 34000,
  "recorded_at": "2026-04-02T10:40:00+07:00"
}
```

### Notes
- price fields may be `null` if no valid prices were available.
- this contract is intended for history panels and chart/table feeds later.

---

## 7. KeywordDetail JSON

### Purpose
Used for the TUI detail screen and future web detail page.

### Shape

```json
{
  "keyword": {
    "id": "kw_001",
    "keyword": "minyak goreng 2L",
    "basic_filter": "2L",
    "threshold_price": 25000,
    "interval_minutes": 5,
    "telegram_enabled": true,
    "status": "active"
  },
  "snapshot": {
    "id": "snap_001",
    "tracked_keyword_id": "kw_001",
    "scan_job_id": "scan_001",
    "min_price": 23800,
    "avg_price": 27500,
    "max_price": 34000,
    "raw_count": 20,
    "grouped_count": 8,
    "signal": "BUY_NOW",
    "snapshot_at": "2026-04-02T10:40:00+07:00"
  },
  "top_deals": [
    {
      "id": "grp_001",
      "group_key": "minyak-goreng-2l-a",
      "representative_title": "Minyak Goreng 2L Promo",
      "representative_seller": "Seller A",
      "best_price": 23800,
      "original_price": 32000,
      "is_promo": true,
      "listing_count": 5,
      "sample_url": "https://example.com/item/123"
    },
    {
      "id": "grp_002",
      "group_key": "minyak-goreng-2l-b",
      "representative_title": "Minyak Goreng 2L Hemat",
      "representative_seller": "Seller B",
      "best_price": 24100,
      "original_price": null,
      "is_promo": false,
      "listing_count": 3,
      "sample_url": "https://example.com/item/456"
    }
  ],
  "recent_events": [
    {
      "id": "evt_001",
      "tracked_keyword_id": "kw_001",
      "scan_job_id": "scan_001",
      "level": "ALERT",
      "event_type": "new_lowest",
      "message": "minyak goreng 2L hit new low: 23.800",
      "payload_json": null,
      "sent_to_telegram": true,
      "created_at": "2026-04-02T10:41:00+07:00"
    }
  ],
  "recent_history": [
    {
      "id": "pp_001",
      "tracked_keyword_id": "kw_001",
      "scan_job_id": "scan_001",
      "min_price": 23800,
      "avg_price": 27500,
      "max_price": 34000,
      "recorded_at": "2026-04-02T10:40:00+07:00"
    },
    {
      "id": "pp_002",
      "tracked_keyword_id": "kw_001",
      "scan_job_id": "scan_000",
      "min_price": 24100,
      "avg_price": 27900,
      "max_price": 33900,
      "recorded_at": "2026-04-02T10:35:00+07:00"
    }
  ]
}
```

### Notes
- this is one of the most important contracts in the system.
- it maps very naturally to both TUI detail rendering and future web detail rendering.

---

## 8. DashboardState JSON

### Purpose
Used for the TUI main dashboard and future dashboard API/page.

### Shape

```json
{
  "tracked_keywords": [
    {
      "id": "kw_001",
      "keyword": "minyak goreng 2L",
      "status": "active",
      "has_new_alert": true
    },
    {
      "id": "kw_002",
      "keyword": "gula pasir 1kg",
      "status": "active",
      "has_new_alert": false
    }
  ],
  "selected_keyword_id": "kw_001",
  "selected_snapshot": {
    "id": "snap_001",
    "tracked_keyword_id": "kw_001",
    "scan_job_id": "scan_001",
    "min_price": 23800,
    "avg_price": 27500,
    "max_price": 34000,
    "raw_count": 20,
    "grouped_count": 8,
    "signal": "BUY_NOW",
    "snapshot_at": "2026-04-02T10:40:00+07:00"
  },
  "top_deals": [
    {
      "id": "grp_001",
      "group_key": "minyak-goreng-2l-a",
      "representative_title": "Minyak Goreng 2L Promo",
      "representative_seller": "Seller A",
      "best_price": 23800,
      "original_price": 32000,
      "is_promo": true,
      "listing_count": 5,
      "sample_url": "https://example.com/item/123"
    }
  ],
  "recent_events": [
    {
      "id": "evt_001",
      "tracked_keyword_id": "kw_001",
      "scan_job_id": "scan_001",
      "level": "ALERT",
      "event_type": "new_lowest",
      "message": "minyak goreng 2L hit new low: 23.800",
      "payload_json": null,
      "sent_to_telegram": true,
      "created_at": "2026-04-02T10:41:00+07:00"
    }
  ],
  "runtime_status": {
    "accepting_new_work": true,
    "running_count": 1,
    "max_concurrent": 2,
    "reconciled_running_jobs": 1,
    "last_reconciled_at": "2026-04-02T10:39:55+07:00",
    "pruned_raw_listings": 9,
    "last_pruned_at": "2026-04-02T10:39:55+07:00",
    "pruned_alert_events": 5,
    "last_alert_pruned_at": "2026-04-02T10:39:55+07:00"
  }
}
```

### Notes
- `tracked_keywords` here is intentionally a lighter summary shape, not full TrackedKeyword JSON.
- this contract is optimized for dashboard rendering speed and clarity.
- `runtime_status` may be `null` if runtime status is unavailable in a given app context.

---

## 9. RuntimeStatusSummary JSON

### Purpose
Used to surface lightweight operational/runtime state to dashboard consumers without exposing runtime internals directly.

### Shape

```json
{
  "accepting_new_work": true,
  "running_count": 1,
  "max_concurrent": 2,
  "reconciled_running_jobs": 1,
  "last_reconciled_at": "2026-04-02T10:39:55+07:00",
  "pruned_raw_listings": 9,
  "last_pruned_at": "2026-04-02T10:39:55+07:00",
  "pruned_alert_events": 5,
  "last_alert_pruned_at": "2026-04-02T10:39:55+07:00"
}
```

### Notes
- this is intentionally a concise operational summary, not a monitoring framework payload.
- startup maintenance outcomes are surfaced here because they are useful for local debugging and user trust.

---

## 10. WorkerScanResult JSON

### Purpose
Used internally between worker, application layer, and persistence boundary.

### Shape

```json
{
  "tracked_keyword_id": "kw_001",
  "scan_job": {
    "id": "scan_001",
    "tracked_keyword_id": "kw_001",
    "started_at": "2026-04-02T10:39:55+07:00",
    "finished_at": "2026-04-02T10:40:00+07:00",
    "status": "success",
    "error_message": null,
    "raw_count": 20,
    "grouped_count": 8
  },
  "raw_listings": [
    {
      "id": "raw_001",
      "scan_job_id": "scan_001",
      "source": "tokopedia",
      "title": "Minyak Goreng 2L Promo",
      "normalized_title": "minyak goreng 2l",
      "seller_name": "Seller A",
      "price": 23800,
      "original_price": 32000,
      "is_promo": true,
      "url": "https://example.com/item/123",
      "scraped_at": "2026-04-02T10:39:58+07:00"
    }
  ],
  "grouped_listings": [
    {
      "id": "grp_001",
      "scan_job_id": "scan_001",
      "group_key": "minyak-goreng-2l-a",
      "representative_title": "Minyak Goreng 2L Promo",
      "representative_seller": "Seller A",
      "best_price": 23800,
      "original_price": 32000,
      "is_promo": true,
      "listing_count": 5,
      "sample_url": "https://example.com/item/123"
    }
  ],
  "snapshot": {
    "id": "snap_001",
    "tracked_keyword_id": "kw_001",
    "scan_job_id": "scan_001",
    "min_price": 23800,
    "avg_price": 27500,
    "max_price": 34000,
    "raw_count": 20,
    "grouped_count": 8,
    "signal": "BUY_NOW",
    "snapshot_at": "2026-04-02T10:40:00+07:00"
  },
  "events": [
    {
      "id": "evt_001",
      "tracked_keyword_id": "kw_001",
      "scan_job_id": "scan_001",
      "level": "ALERT",
      "event_type": "new_lowest",
      "message": "minyak goreng 2L hit new low: 23.800",
      "payload_json": null,
      "sent_to_telegram": false,
      "created_at": "2026-04-02T10:40:00+07:00"
    }
  ]
}
```

### Notes
- this is primarily an internal contract, but it is useful because it captures a full worker cycle output in one shape.
- it can simplify testing and debugging significantly.

---

## 11. TelegramAlertPayload JSON

### Purpose
Used as a formatting boundary between alert generation and Telegram delivery.

### Shape

```json
{
  "tracked_keyword_id": "kw_001",
  "keyword": "minyak goreng 2L",
  "event_type": "new_lowest",
  "signal": "BUY_NOW",
  "message": "minyak goreng 2L hit new low: 23.800",
  "best_price": 23800,
  "threshold_price": 25000,
  "snapshot_at": "2026-04-02T10:40:00+07:00",
  "top_listing": {
    "representative_title": "Minyak Goreng 2L Promo",
    "representative_seller": "Seller A",
    "best_price": 23800,
    "sample_url": "https://example.com/item/123"
  }
}
```

### Notes
- this keeps Telegram formatting separate from domain logic.
- later other notifiers can adopt similar payload contracts.

---

## Lightweight Summary DTOs

Some screens do not need full entity payloads.

### GroupedListingSummary JSON

```json
{
  "representative_title": "Minyak Goreng 2L Promo",
  "representative_seller": "Seller A",
  "best_price": 23800,
  "is_promo": true
}
```

These lighter forms are recommended for dashboard rendering when full detail is not needed.

---

## Field Ownership Guidance

### Domain entity fields
Belong to core business meaning.
Examples:
- `threshold_price`
- `signal`
- `group_key`
- `listing_count`

### View/DTO fields
Belong to rendering and use case shape.
Examples:
- `has_new_alert`
- `selected_keyword_id`
- `top_deals`
- `recent_events`
- `runtime_status`

### Renderer-only fields
Should not be stored in contracts.
Examples:
- `selected`
- `is_focused`
- `row_color`
- `screen_mode`

---

## Contract Stability Guidance

### Stable contracts for v1
These should be kept relatively stable once implementation starts:
- MarketSnapshot JSON
- AlertEvent JSON
- KeywordDetail JSON
- DashboardState JSON
- RuntimeStatusSummary JSON

### Flexible internal contracts
These can evolve more freely in early development:
- WorkerScanResult JSON
- TelegramAlertPayload JSON

---

## Recommendations for Implementation

### Strong recommendation
Create application DTOs or response models that map from domain entities.

Suggested families:
- `dto/dashboard`
- `dto/detail`
- `dto/alert`
- `dto/worker`

### Avoid
- returning database rows directly
- letting TUI compute its own snapshot shape from raw records
- mixing notifier payload generation into domain entities

---

## Next Recommended Document

After JSON contracts, the next logical step is:

- `docs/project-structure-v1.md`

Then after that:
- `docs/database-schema-v1.md`
- `docs/worker-flow-v1.md`

Recommended order:
1. project structure
2. database schema
3. worker flow
4. alert flow
