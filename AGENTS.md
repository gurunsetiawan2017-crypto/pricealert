# AGENTS.md

## Purpose

This file provides instructions for coding agents working in this repository.

Treat this file and the `docs/` directory as the primary source of truth before making implementation decisions.

This project is still in an architecture-first phase. The goal is to implement it incrementally without breaking the boundaries already agreed on.

---

## Mandatory Reading Order

Before making code changes, read these files in order:

1. `README.md`
2. `docs/plan-v1.md`
3. `docs/domain-model-v1.md`
4. `docs/json-contracts-v1.md`
5. `docs/review-gaps-risks-v1.md`
6. `docs/grouping-strategy-v1.md`
7. `docs/worker-runtime-v1.md`
8. `docs/alert-strategy-v1.md`
9. `docs/database-schema-v1.md`
10. `docs/project-structure-v1.md`
11. `docs/implementation-plan-v1.md`

Do not skip these documents.

---

## Project Summary

PriceAlert is a Go-based local-first marketplace monitoring tool.

Current v1 direction:
- single-user first
- TUI-first experience
- Tokopedia-focused for early validation
- MariaDB as primary storage
- grouped market snapshots instead of raw noisy listing output
- Telegram as the first external alert channel
- future-ready for JSON/API and web interface evolution

The product should behave more like a decision-support tool than a raw price tracker.

---

## Architecture Rules

### 1. Keep domain clean
Domain code must not depend on:
- Bubble Tea / TUI
- Telegram formatting
- SQL-specific details as a design driver
- future web handlers

### 2. Keep raw and grouped data separate
Do not collapse `RawListing` and `GroupedListing` into one simplified model.
Grouping is a first-class trust boundary in this project.

### 3. Snapshots and alerts are first-class outputs
`MarketSnapshot` and `AlertEvent` are not temporary helper objects.
They are meaningful persisted outputs.

### 4. TUI is a consumer, not the source of truth
The TUI must consume DTO/query outputs.
It must not directly read DB rows or own business logic.

### 5. Runtime remains single-process in v1
Do not introduce a separate daemon unless explicitly asked.
For v1:
- one local process
- embedded scheduler/worker
- no overlapping scans for the same tracked keyword

### 6. JSON-ready internal contracts matter
Even if the primary interface is TUI, internal contracts should stay structured and future-ready for API/web use.

---

## Design Priorities

When in doubt, prioritize these in order:

1. correctness of grouped market observations
2. alert trustworthiness and anti-spam behavior
3. clean architecture boundaries
4. runtime safety and recoverability
5. maintainability of the codebase
6. UI polish

Do not optimize the visual layer before the grouped data pipeline is trustworthy.

---

## Scope Control

### In scope for v1
- project skeleton
- MariaDB schema and repositories
- domain model implementation
- grouping engine
- market snapshot logic
- alert logic with anti-spam
- single-process scheduler/worker runtime
- TUI dashboard and detail flow
- Telegram notifier

### Out of scope unless explicitly requested
- multi-user support
- multi-marketplace support
- machine learning / embeddings
- advanced prediction engine
- production-grade daemon/service split
- full web interface
- over-engineered generic rule engines

---

## Implementation Order

Follow `docs/implementation-plan-v1.md`.

Preferred milestone order:
1. project skeleton
2. migrations and DB foundation
3. domain types and enums
4. grouping engine
5. snapshot/history logic
6. alert logic
7. scan orchestration
8. scraper feasibility adapter
9. scheduler/worker runtime
10. DTO/query layer
11. TUI
12. Telegram integration
13. hardening

Do not jump ahead to later milestones unless asked.

---

## Grouping Rules

Grouping is one of the most sensitive parts of the project.

Key rules:
- grouping must be explainable
- under-grouping is safer than over-grouping in v1
- grouped listings drive snapshots and alerts
- raw listings should not directly drive user-facing alert decisions

When implementing grouping:
- normalize titles conservatively
- keep size and bundle differences meaningful
- apply hard mismatch rules first
- avoid opaque heuristics too early

Refer to: `docs/grouping-strategy-v1.md`

---

## Alert Rules

Alert behavior must remain conservative.

Important v1 rules:
- prioritize `threshold_hit` and `new_lowest`
- anti-spam is mandatory
- Telegram should only carry actionable alerts by default
- operational/runtime events belong mostly in the in-app log

Do not implement noisy or highly aggressive alerting.

Refer to: `docs/alert-strategy-v1.md`

---

## Runtime Rules

The worker/runtime model is already decided for v1.

Important rules:
- single local process
- scheduler and worker live in the app runtime
- no overlapping scan for the same tracked keyword
- next run is based on completion time
- persist meaningful state to MariaDB
- TUI must not own scan execution logic

Refer to: `docs/worker-runtime-v1.md`

---

## Database Rules

Use the database schema design as the baseline.

Important rules:
- store raw and grouped records separately
- prefer one snapshot per scan
- keep event history persisted
- plan retention for raw data early
- do not prematurely add many future-only tables

Refer to: `docs/database-schema-v1.md`

---

## Project Structure Rules

Use the project structure from:
- `docs/project-structure-v1.md`

Expected major boundaries:
- `cmd/pricealert`
- `internal/app`
- `internal/config`
- `internal/domain`
- `internal/dto`
- `internal/service`
- `internal/runtime`
- `internal/repository`
- `internal/infra`
- `internal/tui`
- `migrations`

Do not collapse everything into a few giant files.
But also do not create unnecessary micro-packages too early.

---

## Code Style Guidance

### General
- keep code simple and readable
- prefer explicit names over clever abstractions
- add comments where architectural intent is helpful
- keep functions focused
- use interfaces where they support boundaries, not just for ceremony

### Avoid
- giant service objects with mixed responsibilities
- domain structs polluted by UI or SQL concerns
- scraper logic spread across unrelated packages
- TUI code talking directly to repositories
- returning raw DB row structs to UI layers

---

## Testing Guidance

The highest-value tests early are:
- grouping tests
- snapshot logic tests
- alert anti-spam tests
- repository integration checks
- scan workflow tests

UI tests are lower priority than grouped-data correctness tests.

Use `testdata/` for:
- listing title samples
- grouping fixture cases
- parser input samples
- alert-history scenarios

---

## How to Work on Tasks

When given a task:

1. read the relevant docs first
2. restate the architecture constraints briefly
3. identify which milestone the task belongs to
4. make incremental changes only
5. avoid redesign unless necessary
6. explain tradeoffs clearly if a doc and implementation pressure conflict

If a requested change would violate the current architecture, explain the conflict instead of silently drifting the design.

---

## Preferred Behavior for Coding Agents

When starting work, do this:
- summarize what you understood from the docs
- list the files you plan to create or modify
- keep changes small and reviewable
- align code strictly with the documented milestone

When uncertain, prefer:
- minimal scaffolding
- conservative logic
- preserving documented boundaries

---

## Recommended First Task for New Agents

If no specific implementation task is given, begin with Milestone A from `docs/implementation-plan-v1.md`:

- initialize Go project structure
- add config/bootstrap skeleton
- add migrations folder and migration scaffold
- add domain model skeleton and enums
- add repository interfaces only

Do not implement full scraper, TUI, Telegram, or runtime logic yet unless explicitly asked.

---

## Final Reminder

The biggest danger in this project is not lack of ideas.
The biggest danger is building a polished shell around weak marketplace observations.

So always protect these first:
- grouping quality
- snapshot correctness
- alert trustworthiness
- runtime safety

If those remain strong, the rest of the system can evolve cleanly.
