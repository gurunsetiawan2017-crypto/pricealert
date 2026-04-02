# Implementation Plan v1

## Purpose

This document defines the recommended implementation sequence for PriceAlert / DealHunt v1.

The goal is to turn the existing design documents into a practical build order that:

- reduces the highest technical risks early
- avoids wasted UI-first work on an unstable data pipeline
- keeps implementation incremental and testable
- works well for either manual coding or Codex-assisted coding

This plan assumes the current agreed direction:

- single-user local application
- Go + MariaDB
- TUI-first interface
- grouped-market snapshot logic
- Telegram as the first external notifier
- JSON-ready internal contracts

---

## Implementation Strategy

### Core principle
Build from the inside out, not from the screen inward.

That means the order should prioritize:
1. project foundation
2. persistence and schema
3. domain and service logic
4. grouping and snapshot quality
5. alert behavior
6. runtime scheduling
7. TUI wiring
8. notifier integration

### Why this order is important
The biggest product risk is not whether the UI looks good.
The biggest product risk is whether the system can produce trustworthy grouped market observations and alerts.

So the implementation should validate the data pipeline first.

---

## Phase 0 — Project Foundation

### Goal
Create the minimum project skeleton so implementation can proceed in a clean direction.

### Tasks
- initialize Go module
- create base folder structure according to `docs/project-structure-v1.md`
- add README with short architecture summary
- add configuration loading skeleton
- add DB connection skeleton
- add migration folder and migration runner approach

### Expected outcome
A compilable but mostly empty project structure with clear boundaries.

### Deliverables
- `go.mod`
- `cmd/pricealert/main.go`
- initial `internal/` directories
- `migrations/`
- config/bootstrap stubs

### Notes
Do not build TUI screens yet beyond perhaps a placeholder boot screen.

---

## Phase 1 — Database and Persistence Foundation

### Goal
Make the persistence layer real early, because runtime, alert history, and dashboard state all depend on it.

### Tasks
- implement first MariaDB migration(s)
- create DB connection and transaction helpers
- create repository interfaces
- implement MariaDB repositories for core entities

### Minimum schema to start with
The practical minimum starting set is:
- `tracked_keywords`
- `scan_jobs`
- `raw_listings`
- `grouped_listings`
- `market_snapshots`
- `alert_events`

### Optional in this phase or next
- `price_points`
- `alert_rules`

### Expected outcome
The app can persist and retrieve domain-relevant state without depending on TUI.

### Deliverables
- initial SQL migrations
- repository interfaces
- MariaDB repository implementations
- simple integration checks

### Risk reduction achieved
- validates DB assumptions early
- avoids later domain/DB mismatch

---

## Phase 2 — Domain Models and Enums

### Goal
Turn the documented model into concrete Go types.

### Tasks
- implement domain structs
- implement enums/constants
- add minimal domain validation where appropriate
- ensure contracts align with the docs

### Core types to implement first
- `TrackedKeyword`
- `ScanJob`
- `RawListing`
- `GroupedListing`
- `MarketSnapshot`
- `AlertEvent`

### Add next
- `PricePoint`
- `AlertRule`

### Expected outcome
The rest of the code can work against stable business objects instead of ad hoc maps or DB-shaped structs.

### Deliverables
- `internal/domain/models.go` or equivalent split files
- `internal/domain/enums.go`

---

## Phase 3 — Grouping Engine (Highest Data Trust Priority)

### Goal
Implement the grouping logic early because it directly affects snapshot quality and alert trustworthiness.

### Tasks
- implement title normalization
- implement stopword handling
- implement lightweight attribute extraction
- implement hard mismatch rules
- implement similarity scoring
- implement representative listing selection

### Use the grouping strategy doc as source of truth
Refer directly to:
- `docs/grouping-strategy-v1.md`

### Recommended development approach
Start with deterministic, explainable rules.
Do not introduce ML or advanced fuzzy matching complexity.

### Expected outcome
Given raw listings, the system can produce grouped listings that are good enough for downstream use.

### Deliverables
- grouping service
- normalization helpers
- test fixture set in `testdata/`
- grouping unit tests

### Risk reduction achieved
- reduces the biggest correctness risk early
- makes snapshot and alert behavior much safer to implement next

---

## Phase 4 — Snapshot and History Logic

### Goal
Produce stable grouped-market summaries from grouped results.

### Tasks
- compute grouped min/avg/max
- assign grouped_count and raw_count
- determine signal value
- create `MarketSnapshot`
- create `PricePoint` entries

### Semantics to preserve
Use the definitions locked in docs:
- snapshot is based on grouped listings
- not raw listing noise

### Expected outcome
The system can summarize a scan into a reliable snapshot and history point.

### Deliverables
- snapshot service
- history service
- snapshot tests

---

## Phase 5 — Alert Logic and Anti-Spam

### Goal
Implement the first trustworthy buying alerts.

### Tasks
- implement threshold_hit logic
- implement new_lowest logic
- optionally defer price_drop if needed
- implement cooldown logic
- implement meaningful-improvement rule
- persist alert events

### Use this doc as source of truth
- `docs/alert-strategy-v1.md`

### Recommended v1 scope
Must-have:
- threshold_hit
- new_lowest
- cooldown
- no duplicate same-price alert

### Expected outcome
The system can emit actionable alerts without spamming the user every scan.

### Deliverables
- alert service
- alert tests
- event generation logic

### Risk reduction achieved
- preserves user trust
- prevents a common “works but unusable” failure mode

---

## Phase 6 — Scan Execution Service

### Goal
Create the main orchestration flow for one keyword scan.

### Tasks
- create ScanJob with `running`
- call scraper adapter
- persist raw listings
- run grouping service
- persist grouped listings
- compute snapshot/history
- evaluate alerts
- persist events
- mark ScanJob success or failure

### Important note
This phase can be developed before full scheduler/runtime logic.
It can first be invoked manually or from tests.

### Expected outcome
A single end-to-end scan can run in isolation for one tracked keyword.

### Deliverables
- scan service
- integration tests for scan workflow

---

## Phase 7 — Scraper Feasibility and Source Adapter

### Goal
Validate the highest external uncertainty: Tokopedia data acquisition.

### Tasks
- determine practical fetch strategy
- confirm reliable extraction of:
  - title
  - seller
  - price
  - promo/original price if available
  - URL
- capture representative sample inputs for tests
- isolate scraper implementation in infra

### Important note
This phase may partially run earlier in parallel as a feasibility spike.
In fact, that is often a good idea.

### Recommendation
Do an early spike as soon as possible, even before full TUI work.

### Expected outcome
The application has a usable source adapter, or the feasibility risks are known early.

### Deliverables
- scraper adapter interface
- Tokopedia-specific implementation spike or MVP
- sample testdata

### Risk reduction achieved
- validates the single largest real-world dependency

---

## Phase 8 — Runtime Scheduler and Worker

### Goal
Turn the single-scan workflow into a local continuous monitoring loop.

### Tasks
- implement scheduler state
- implement eligible-run calculation
- implement non-overlap per keyword
- implement concurrency limit across keywords
- implement graceful startup/shutdown behavior
- implement manual refresh integration

### Use this doc as source of truth
- `docs/worker-runtime-v1.md`

### Expected outcome
The system can keep monitoring tracked keywords over time inside one local process.

### Deliverables
- scheduler
- worker executor
- runtime state management
- runtime tests where practical

---

## Phase 9 — Query DTOs and View Models

### Goal
Prepare application-facing outputs for TUI and future JSON/API use.

### Tasks
- implement `DashboardState` DTO
- implement `KeywordDetail` DTO
- implement alert/event view DTOs
- implement mapping/query services

### Why now
By this point the data pipeline is already real.
The TUI can now consume stable view models instead of half-finished repository logic.

### Expected outcome
The application can produce clean boundary objects for rendering.

### Deliverables
- DTO definitions
- query service
- mapping tests

---

## Phase 10 — TUI Skeleton and Main Screens

### Goal
Add the first usable terminal experience once the data pipeline is already meaningful.

### Tasks
- implement TUI app bootstrap
- implement dashboard screen
- implement keyword selection behavior
- implement detail screen
- implement add/edit/delete flows
- implement log panel rendering
- implement refresh action wiring

### Important note
The TUI should consume DTOs/services, not repositories directly.

### Expected outcome
A user can launch the app, inspect tracked keywords, and see live/persisted state.

### Deliverables
- Bubble Tea app skeleton
- dashboard view
- detail view
- form/modal flows

---

## Phase 11 — Telegram Notifier Integration

### Goal
Enable first external actionable alert delivery.

### Tasks
- implement Telegram sender adapter
- map alert DTOs to notifier payloads
- send only buying alerts
- persist/send delivery events clearly

### Important note
Telegram integration should not pollute domain logic.
Keep it behind notifier interfaces.

### Expected outcome
Actionable alerts can be sent outside the terminal.

### Deliverables
- Telegram adapter
- notifier payload mapping
- delivery success/failure event handling

---

## Phase 12 — Usability and Hardening

### Goal
Make the app less fragile and more comfortable after the main flow already works.

### Tasks
- improve empty/loading/error states
- refine freshness/staleness indicators
- improve logging clarity
- tune anti-spam defaults
- add retention/pruning strategy
- add more tests around grouping and alerts
- add developer scripts and local setup improvements

### Expected outcome
The first usable version becomes more trustworthy and maintainable.

---

## Recommended Testing Strategy by Phase

### Early phases
Prioritize:
- grouping tests
- snapshot tests
- alert logic tests
- repository integration checks

### Mid phases
Add:
- scan workflow tests
- runtime scheduling tests where feasible

### Later phases
Add:
- TUI behavior checks where useful
- notifier adapter tests

### Important note
The highest-value tests are not UI tests first.
They are correctness tests for:
- grouping
- snapshot semantics
- alert anti-spam behavior

---

## Suggested Deliverable Milestones

## Milestone A — Foundation
Includes:
- project skeleton
- config/bootstrap
- migrations
- DB connection
- domain model skeleton

### Success criteria
Project compiles and migrations can run.

---

## Milestone B — Trusted Data Core
Includes:
- repositories
- grouping service
- snapshot service
- alert service
- scan service

### Success criteria
A manual or test-triggered scan can produce:
- grouped listings
- snapshot
- alert events

This is one of the most important milestones.

---

## Milestone C — Continuous Runtime
Includes:
- scheduler
- worker loop
- non-overlap behavior
- persisted runtime outcomes

### Success criteria
Tracked keywords can monitor over time in one process without duplicated scan overlap.

---

## Milestone D — Usable TUI
Includes:
- dashboard
- detail screen
- add/edit/delete keyword flow
- live log rendering

### Success criteria
A user can operate the app without touching internal debug tools.

---

## Milestone E — External Alerts
Includes:
- Telegram integration
- delivery event recording
- alert payload formatting

### Success criteria
Useful buying alerts arrive outside the terminal.

---

## Anti-Patterns to Avoid During Implementation

### 1. Building TUI first
Risk:
Creates a polished shell around unstable core logic.

### 2. Coupling scraper code into services directly
Risk:
Makes source-specific complexity leak everywhere.

### 3. Returning DB rows directly to UI
Risk:
Makes future evolution painful.

### 4. Implementing advanced rule engines too early
Risk:
Over-engineering before the simple use case is proven.

### 5. Trying to solve all marketplaces or all user types at once
Risk:
Loses focus and slows real validation.

---

## Recommended First Codex Prompt Direction

When using Codex, a good initial instruction is:

1. read all docs in `/docs`
2. treat them as source of truth
3. create project skeleton first
4. do not implement full scraper yet
5. start with migrations, domain types, repository interfaces, and grouping service scaffolding

This reduces the chance that implementation starts from the wrong layer.

---

## Final Recommended Sequence Summary

The best overall sequence is:

1. project skeleton
2. migrations and DB layer
3. domain types
4. grouping engine
5. snapshot/history logic
6. alert logic
7. scan orchestration
8. scraper feasibility adapter
9. scheduler/worker runtime
10. DTO/query layer
11. TUI
12. Telegram integration
13. hardening and cleanup

---

## Final Summary

The implementation plan should reduce uncertainty in the most dangerous parts first.

That means:
- data quality before UI polish
- grouped logic before visual dashboards
- anti-spam before external notifications
- runtime clarity before background automation grows more complex

If this build order is followed, the project will have a much better chance of becoming both useful and maintainable instead of just visually interesting.
