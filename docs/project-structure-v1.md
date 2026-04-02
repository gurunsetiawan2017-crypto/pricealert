# Project Structure v1

## Purpose

This document defines the recommended Go project structure for PriceAlert / DealHunt v1.

The goal is to create a structure that is:

- clean enough for long-term growth
- practical enough for a first implementation
- aligned with the documented domain model
- aligned with the single-process runtime decision
- compatible with TUI-first UX and future web/API evolution

This is a design document, not a final code generation spec.
Its role is to define boundaries and responsibilities before implementation starts.

---

## High-Level Principles

### 1. Separate domain from delivery
Core business concepts and behavior should not depend on:
- Bubble Tea / TUI concerns
- Telegram formatting
- MariaDB SQL details
- future web handlers

### 2. Keep the runtime single-process, not single-layer
Even though v1 runs in one local process, code structure should still separate:
- domain
- application services
- infrastructure
- interface layer

### 3. Optimize for clarity first
The project should be easy to understand and evolve.
Avoid structures that look “enterprise-ready” but slow down early development.

### 4. Use DTO/view models at boundaries
Do not let TUI read raw DB rows directly.
Do not let domain entities become notifier payloads directly.
Use explicit mappers where needed.

### 5. Respect the selected architecture
The structure should support:
- TUI main interface
- worker + scheduler inside the app runtime
- MariaDB persistence
- JSON-ready contracts
- Telegram alerts

---

## Recommended Overall Layout

A strong v1 layout is:

```text
pricealert/
  ├── cmd/
  │   └── pricealert/
  │       └── main.go
  │
  ├── internal/
  │   ├── app/
  │   ├── domain/
  │   ├── service/
  │   ├── runtime/
  │   ├── repository/
  │   ├── infra/
  │   ├── tui/
  │   ├── dto/
  │   └── config/
  │
  ├── migrations/
  ├── docs/
  ├── scripts/
  ├── testdata/
  ├── go.mod
  └── README.md
```

### Why this is a good fit
- `cmd/` keeps the entrypoint clean
- `internal/` protects implementation details
- domain and infrastructure stay clearly separated
- TUI can evolve without corrupting business logic
- future web/API can be added without major reorganization

---

## Directory Responsibilities

## 1. cmd/pricealert

### Purpose
Contains the application entrypoint.

### Responsibilities
- load configuration
- initialize DB and dependencies
- assemble application services
- start runtime and TUI

### Example
```text
cmd/
  └── pricealert/
      └── main.go
```

### Important note
Keep `main.go` thin.
It should assemble the app, not contain product logic.

---

## 2. internal/domain

### Purpose
Contains core business entities, enums, and domain-facing logic that should remain stable regardless of transport or UI.

### Recommended substructure

```text
internal/domain/
  ├── trackedkeyword/
  ├── scanjob/
  ├── listing/
  ├── snapshot/
  ├── alert/
  └── common/
```

### Likely contents
- entities / structs
- enums
- small domain invariants
- value objects if needed

### Example concepts
- `TrackedKeyword`
- `ScanJob`
- `RawListing`
- `GroupedListing`
- `MarketSnapshot`
- `PricePoint`
- `AlertRule`
- `AlertEvent`

### Important note
Do not place SQL, Bubble Tea, or Telegram formatting here.

---

## 3. internal/dto

### Purpose
Contains JSON-ready and UI-ready data contracts that sit at application boundaries.

### Recommended substructure

```text
internal/dto/
  ├── dashboard/
  ├── detail/
  ├── alert/
  ├── worker/
  └── common/
```

### Likely contents
- `DashboardState`
- `KeywordDetail`
- `GroupedListingView`
- `AlertEventView`
- `TelegramAlertPayload`
- `WorkerScanResult`

### Why this matters
This keeps the system ready for:
- TUI rendering
- JSON output
- future web API
- testable contracts

---

## 4. internal/service

### Purpose
Contains core application/business services.

These services orchestrate domain behavior and repositories.

### Recommended substructure

```text
internal/service/
  ├── keyword/
  ├── scan/
  ├── grouping/
  ├── snapshot/
  ├── alert/
  ├── history/
  └── query/
```

### Service responsibilities

#### keyword service
- add keyword
- edit keyword
- pause/resume keyword
- delete keyword

#### scan service
- execute a full scan lifecycle
- create scan job
- fetch raw data through scraper interface
- invoke grouping
- compute snapshot
- persist results
- emit events

#### grouping service
- normalize title
- extract lightweight attributes
- apply hard mismatch rules
- compute similarity
- group listings
- select representative listing

#### snapshot service
- compute grouped min/avg/max
- assign signal
- build MarketSnapshot

#### alert service
- evaluate threshold/new-lowest/price-drop
- enforce anti-spam
- emit alert events
- call notifier adapters

#### history service
- build/store price points
- fetch historical comparisons

#### query service
- compose dashboard/detail DTOs
- map domain/repository results into TUI/web-ready shapes

### Important note
These services are the heart of the app.
They should stay independent from TUI-specific rendering concerns.

---

## 5. internal/runtime

### Purpose
Contains the local runtime execution model decided for v1.

### Recommended substructure

```text
internal/runtime/
  ├── scheduler/
  ├── worker/
  ├── lifecycle/
  └── state/
```

### Responsibilities

#### scheduler
- determine eligible keywords
- respect pause state
- avoid overlap
- dispatch scans

#### worker
- run scan tasks
- coordinate scan execution service calls

#### lifecycle
- startup
- graceful shutdown
- restart behavior

#### state
- in-memory runtime coordination state
- current running flags
- next eligible times
- freshness metadata if needed

### Important note
Runtime coordination state is not the same as domain persistence.
It is operational state only.

---

## 6. internal/repository

### Purpose
Contains repository interfaces and/or implementations for persistence access.

### Recommended substructure

```text
internal/repository/
  ├── interfaces/
  ├── mariadb/
  └── transaction/
```

### Suggested pattern
Use repository interfaces close to the services that need them, then implement them in MariaDB-specific packages.

### Example repository areas
- tracked keywords
- scan jobs
- raw listings
- grouped listings
- market snapshots
- price points
- alert rules
- alert events

### Responsibilities
- persist/fetch domain data
- support query patterns needed by services
- isolate SQL details from business logic

### Important note
Do not let repositories return random DB-shaped structs into the rest of the app.
Prefer domain objects or explicit internal record mappings.

---

## 7. internal/infra

### Purpose
Contains external system adapters and infrastructure implementations.

### Recommended substructure

```text
internal/infra/
  ├── db/
  ├── scraper/
  ├── notifier/
  ├── clock/
  ├── idgen/
  └── logger/
```

### Responsibilities

#### db
- DB connection creation
- low-level DB utilities
- transaction helpers

#### scraper
- Tokopedia-specific fetch and parse adapters
- raw response acquisition
- scraper interfaces/clients

#### notifier
- Telegram sender
- future other notifiers

#### clock
- time abstraction if needed for testing

#### idgen
- ULID/UUID generator abstraction

#### logger
- structured logging helpers if needed

### Important note
External integrations belong here, not in domain packages.

---

## 8. internal/tui

### Purpose
Contains Bubble Tea / TUI-specific UI code.

### Recommended substructure

```text
internal/tui/
  ├── app/
  ├── screen/
  ├── component/
  ├── model/
  ├── update/
  └── view/
```

### Suggested responsibilities

#### app
- TUI application bootstrap
- wiring app model and update loop

#### screen
- dashboard screen
- detail screen
- modal screens

#### component
- reusable UI components
- list blocks
- status bars
- log panel

#### model
- TUI state only
- selection state
- focus state
- current screen state

#### update
- keyboard event handling
- screen transitions
- action dispatch to services

#### view
- rendering functions
- layout formatting

### Important note
TUI state should not become business state.
Examples of TUI-only state:
- selected keyword row
- focused panel
- active modal

These must stay in `internal/tui`, not in domain.

---

## 9. internal/app

### Purpose
Contains application assembly and high-level orchestration.

### Responsibilities
- initialize services and repositories
- wire runtime, TUI, and infrastructure together
- expose startup object or root app composition

### Suggested contents
- dependency wiring
- application constructor
- bootstrapping helpers

### Why keep this separate
This prevents `main.go` from becoming a giant setup file.

---

## 10. internal/config

### Purpose
Contains configuration loading and validation.

### Responsibilities
- environment/config file reading
- DB config
- Telegram config
- runtime tuning config
- defaults and validation

### Examples
- MariaDB DSN
- Telegram bot token / chat id
- scheduler concurrency limit
- optional log level

---

## 11. migrations

### Purpose
Contains SQL migrations for MariaDB schema.

### Suggested contents
- versioned SQL migration files
- schema initialization scripts if needed

### Recommendation
Keep migrations simple and explicit.
Do not hide schema evolution inside application startup logic too early.

---

## 12. scripts

### Purpose
Contains project helper scripts.

### Examples
- local DB setup
- run migrations
- dev run helpers
- test fixtures loader

This is optional but useful for local developer workflow.

---

## 13. testdata

### Purpose
Contains manual listing examples and fixture data useful for:
- grouping tests
- parser tests
- alert logic tests

### Recommendation
This directory will be very valuable for the grouping strategy and scraping validation.

---

## Suggested Package Interaction

A healthy v1 flow looks like this:

```text
TUI -> app/service layer -> repositories / infra
                         -> runtime scheduler/worker

runtime worker -> scan service -> scraper adapter
                             -> grouping service
                             -> snapshot service
                             -> alert service
                             -> repositories

query service -> repositories -> DTO mapping -> TUI
```

### Important rule
TUI should call services, not repositories directly.

---

## Suggested Implementation Boundaries

### Domain layer
Knows about:
- entities
- enums
- core meaning

Does not know about:
- SQL
- TUI
- Telegram
- Bubble Tea

### Service layer
Knows about:
- use cases
- orchestration
- domain rules
- repository interfaces
- DTO creation

Does not know about:
- terminal layout
- SQL details

### Infrastructure layer
Knows about:
- MariaDB
- Tokopedia scraping
- Telegram API
- time/id generation

### TUI layer
Knows about:
- keyboard input
- layout
- screen transitions
- service calls
- DTO display

---

## Recommended v1 Simplification

To avoid over-engineering, the project can start with fewer packages while preserving the same boundaries.

### Minimal but good v1 structure

```text
internal/
  ├── domain/
  ├── service/
  ├── repository/
  ├── infra/
  ├── runtime/
  ├── tui/
  ├── dto/
  ├── app/
  └── config/
```

This is enough.
There is no need to split into too many micro-packages on day one.

The subdirectories can grow as the codebase grows.

---

## Example Early File Layout

A practical early version could look like:

```text
cmd/pricealert/main.go

internal/config/config.go
internal/app/bootstrap.go

internal/domain/models.go
internal/domain/enums.go

internal/dto/dashboard.go
internal/dto/detail.go
internal/dto/alert.go

internal/service/keyword_service.go
internal/service/scan_service.go
internal/service/grouping_service.go
internal/service/snapshot_service.go
internal/service/alert_service.go
internal/service/query_service.go

internal/runtime/scheduler.go
internal/runtime/worker.go
internal/runtime/state.go

internal/repository/interfaces.go
internal/repository/mariadb_keyword_repository.go
internal/repository/mariadb_scan_repository.go
internal/repository/mariadb_snapshot_repository.go
internal/repository/mariadb_alert_repository.go

internal/infra/db/connection.go
internal/infra/scraper/tokopedia.go
internal/infra/notifier/telegram.go
internal/infra/idgen/ulid.go

internal/tui/app.go
internal/tui/model.go
internal/tui/update.go
internal/tui/view.go

migrations/001_init.sql
migrations/002_alert_rules.sql
```

### Why this is a good starting point
It is small enough to build quickly, but the boundaries are already healthy.

---

## Package Risks to Avoid

### 1. God-package service layer
Risk:
Everything gets dumped into one `service` file.

Prevention:
Split by responsibility early enough.

### 2. TUI reaching into repositories
Risk:
UI becomes tightly coupled to persistence structure.

Prevention:
Use query service / DTO mapping.

### 3. Scraper logic spread across services
Risk:
Source-specific parsing leaks everywhere.

Prevention:
Keep scraper-specific concerns in infra scraper package.

### 4. Domain polluted by framework concerns
Risk:
Entities gain Bubble Tea, JSON-view, or SQL-driven design noise.

Prevention:
Keep domain clean and boring.

### 5. Runtime logic hidden inside TUI update loop
Risk:
Hard-to-debug coupling between rendering and scanning.

Prevention:
Keep scheduler/worker independent.

---

## How This Structure Supports Future Web Interface

This structure makes future expansion easier because:

- TUI is only one interface consumer
- DTOs already exist
- domain/services remain UI-neutral
- repositories and infrastructure are already separated

When web is added later, a new package can be introduced such as:

```text
internal/web/
  ├── handler/
  ├── router/
  └── response/
```

without needing to redesign the whole project.

---

## Recommended Next Coding Sequence

Once implementation starts, a sensible order is:

1. config + bootstrap
2. DB connection + migrations
3. domain entities and enums
4. repository interfaces and MariaDB implementations
5. grouping service
6. snapshot service
7. alert service
8. scan service
9. runtime scheduler/worker
10. query service DTOs
11. TUI wiring
12. Telegram notifier integration

This keeps the highest-risk data pipeline pieces earlier than UI polish.

---

## Suggested Next Documents

After project structure, the most useful remaining design docs are:

1. `docs/scraping-feasibility-notes.md`
2. `docs/implementation-plan-v1.md`

Optional after that:
- `docs/test-strategy-v1.md`
- `docs/dev-setup-v1.md`

---

## Final Summary

The best v1 project structure is one that keeps the codebase easy to reason about while preserving the architectural boundaries we already agreed on.

The key idea is:

- domain stays clean
- services orchestrate
- runtime schedules and executes
- infra handles external systems
- TUI renders DTOs instead of raw database shape

If those boundaries stay intact, Codex or manual implementation will have a much easier time building the project without producing a tangled codebase.
