# Worker Runtime v1

## Purpose

This document defines the initial runtime and worker execution model for PriceAlert / DealHunt v1.

The main goal is to make the application behavior clear before implementation becomes too coupled to a specific process model.

This document answers questions such as:

- how scanning runs over time
- how the TUI interacts with monitoring
- what happens when scans are slow
- how failures are handled
- how state is persisted and resumed

---

## High-Level Runtime Decision

### Recommended v1 runtime model
For v1, use a **single local application runtime** with these characteristics:

- local single-user application
- one process started by the user
- worker loop lives inside the application runtime
- state is persisted to MariaDB
- TUI is one consumer of persisted/application state
- no separate daemon process in v1

### Why this is recommended
This keeps v1 simpler and avoids premature complexity around:
- daemon lifecycle management
- IPC between TUI and background service
- local service installation
- process orchestration

The system can still evolve later into:
- separate worker service
- API server
- web UI
- daemonized background mode

But v1 should not start there.

---

## Core Runtime Philosophy

The runtime should behave like this:

1. user launches the application
2. app loads tracked keywords and recent state
3. scheduler determines which keywords need scanning
4. worker executes scans over time
5. results are persisted
6. TUI reads and displays current state
7. alerts are emitted when rules are satisfied

This means:
- TUI is not the source of truth
- database plus application services define the current state
- worker is responsible for updating observation state

---

## Process Model

### v1 process model
One application process contains:

- TUI loop
- scheduler loop
- scan worker(s)
- repositories / DB access
- alert dispatch

### Conceptual structure

```text
Application Process
  ├── TUI Layer
  ├── App Services
  ├── Scheduler
  ├── Scan Workers
  ├── Alert Dispatcher
  └── MariaDB Persistence
```

### Important note
Even though all these live in one process in v1, they should still be separated conceptually in code.

Do not let “single process” become “single tangled module.”

---

## Runtime Modes

Recommended v1 runtime modes:

### 1. Interactive mode
The main intended mode.

Behavior:
- launches TUI
- scheduler and worker loop run in the same app process
- user can observe live state
- user can add/edit/delete tracked keywords

### 2. Headless future possibility
Not required in v1, but architecture should not block it.

Future possibilities:
- worker-only mode
- API-only mode
- daemon mode

Do not implement these now unless truly needed.

---

## Startup Flow

### Recommended startup flow

1. application starts
2. establish DB connection
3. load tracked keywords with status `active` or `paused`
4. initialize in-memory scheduler state
5. load recent snapshots and events for dashboard rendering
6. start scheduler loop
7. start TUI event loop

### Startup goals
- app should become usable quickly
- UI should not block until all scans complete
- existing persisted state should be visible immediately

### Recommendation
Show last known state first, then allow fresh scans to update it asynchronously.

This gives a better user experience than forcing full refresh before rendering.

---

## Shutdown Flow

### Recommended shutdown behavior
When the user exits:

1. stop accepting new scan scheduling
2. allow active scan jobs to finish or stop gracefully
3. flush pending state/event writes
4. close DB connection
5. exit cleanly

### Important note
If graceful shutdown takes too long, the app may allow a timeout and exit with clear logs.

---

## Scheduler Model

## Recommended v1 scheduling model
Each active tracked keyword has its own scheduling state.

The scheduler should determine:
- whether keyword is active
- when it last ran
- whether it is currently running
- when it is next eligible to run

### Suggested scheduling policy
For each tracked keyword:
- do not run if status is `paused`
- do not run if already scanning
- after a scan finishes, compute next eligible run time based on completion time
- optional jitter may be added to reduce robotic behavior

### Why completion-time scheduling is recommended
It is simpler than start-time scheduling for v1.
It avoids immediate overlap pressure when scans run long.

---

## Interval Semantics

### Recommended semantics
`interval_minutes` means approximate time between **completed scan** and **next eligible scan**.

### Example
If interval is 5 minutes:
- scan finishes at 10:10
- next run becomes eligible around 10:15

### Optional jitter
A small random jitter can be added, for example:
- 0 to 30 seconds
- or 0 to 10 percent of interval

Jitter is helpful for:
- reducing fixed robotic timing
- smoothing load slightly

---

## Scan Concurrency Policy

### Recommended v1 rule
For a single tracked keyword:
- never allow overlapping scans

For multiple tracked keywords:
- allow limited concurrency across different keywords
- cap total concurrent scans to a safe number

### Why this matters
Without this rule, short intervals plus slow scans can cause:
- duplicate work
- stale data races
- alert duplication
- resource waste

### Suggested v1 concurrency limit
Keep it conservative.
For example:
- 1 to 3 concurrent keyword scans globally

The exact number depends on scraper reliability and request behavior.

---

## Overlap Handling

### Problem
What if the next interval arrives but the current scan is still running?

### Recommended v1 behavior
For the same keyword:
- skip overlapping start
- mark next run when current one completes

### Simpler rule
No queued overlap stack.
No backlog accumulation.

This is safer and easier to reason about in v1.

---

## Scan Job Lifecycle

### Recommended lifecycle

1. scheduler selects eligible tracked keyword
2. create ScanJob with status `running`
3. start scraping/fetching
4. parse raw data
5. normalize and group
6. compute snapshot
7. compute history point
8. evaluate alerts
9. persist results
10. mark ScanJob as `success` or `failed`
11. emit events
12. release keyword runtime lock

### On failure
If scan fails:
- mark ScanJob as `failed`
- save error message
- emit failure event
- do not crash entire runtime

---

## Persistence Model During Runtime

### Recommended rule
Persist important state changes, do not keep them only in memory.

Persist at least:
- tracked keyword config changes
- ScanJob creation and completion
- grouped results
- snapshot
- history point
- alert events

### Why
This ensures:
- TUI can reconstruct state after restart
- future headless/API modes are easier
- runtime remains resilient to app restarts

---

## TUI and Worker Interaction

### Recommended principle
The TUI should not perform scanning directly.

The TUI should:
- display current state
- dispatch user actions
- subscribe to application state updates

The worker/scheduler should:
- decide when to scan
- execute monitoring logic
- persist results

### Good separation
TUI triggers intent such as:
- add keyword
- pause keyword
- refresh now

Application services then interpret and execute that intent.

---

## Manual Refresh Behavior

### Recommended behavior
If user presses refresh on a selected keyword:
- request immediate scan if not already running
- if already running, show status instead of starting duplicate work

### Why
This preserves single-scan-per-keyword discipline.

---

## Pause and Resume Behavior

### Pause
When a keyword is paused:
- scheduler ignores it
- no new scans start
- existing state remains visible
- history remains intact

### Resume
When resumed:
- keyword becomes eligible for scheduling again
- optional immediate scan can be triggered

### Recommendation
For better UX, resume may trigger a near-immediate scan.

---

## Error Handling Philosophy

### Important principle
Scan failure for one keyword must not crash the whole application.

### Recommended failure isolation
Failures should be isolated at the scan-job level.

Examples:
- one keyword fetch fails
- parser fails for one page
- Telegram send fails

These should create events and logs, but not kill the main runtime.

---

## Failure Types to Handle

### 1. DB connection failure at startup
#### Behavior
- app should fail fast with clear error
- TUI should not pretend system is healthy

### 2. Scan fetch failure
#### Behavior
- mark ScanJob failed
- emit `scan_failed`
- retry naturally on next interval

### 3. Partial parse failure
#### Behavior
- prefer failing the scan explicitly unless partial success behavior is deliberately supported

### 4. Telegram send failure
#### Behavior
- keep scan successful if main scan succeeded
- emit notifier failure event separately

### 5. Slow scan duration
#### Behavior
- no overlap for same keyword
- next run based on completion time

---

## State Freshness

### Problem
TUI may show stale data if no successful scan has happened recently.

### Recommendation
Track freshness using timestamps.

At minimum the UI should be able to show:
- last successful snapshot time
- whether current data is stale beyond interval expectation

### Suggested future-friendly idea
A state can be interpreted as:
- fresh
- stale
- syncing
- failed

This does not need to be a domain enum yet, but the concept is useful.

---

## Local Runtime Locking

### Purpose
Prevent duplicate runtime work for the same tracked keyword.

### Recommendation
Maintain lightweight in-memory runtime state per keyword:
- currently running or not
- next eligible run time
- last attempt time
- last success time

This is runtime coordination state, not domain data.

### Important note
Do not confuse runtime scheduling state with persisted business entities.

---

## Recovery After Restart

### Recommended v1 behavior
On app restart:
- load persisted tracked keywords
- load recent snapshots/events
- ignore old in-memory runtime state
- rebuild scheduler state from persisted data
- continue scanning based on current time and status

### Recommendation
If a ScanJob was left in `running` state due to crash, treat it as abandoned during startup reconciliation.
Possible action:
- mark it as failed or stale-recovered

This behavior should be explicit in implementation.

---

## Refresh-on-Start Policy

### Open question
Should the app immediately scan everything on startup?

### Recommended v1 policy
Not always.
Use a freshness-based approach.

Examples:
- if no snapshot exists -> scan soon
- if snapshot is too old -> scan soon
- if snapshot is still recent -> normal interval behavior is fine

### Why
This avoids needless startup spikes and makes the app more stable.

---

## Event Emission During Runtime

### Recommended event categories
- `scan_started`
- `scan_completed`
- `scan_failed`
- `threshold_hit`
- `new_lowest`
- `telegram_sent`
- `telegram_failed`
- `keyword_paused`
- `keyword_resumed`
- `keyword_added`
- `keyword_deleted`

### Recommendation
Emit events for meaningful transitions, not for every tiny internal step.

This keeps logs useful rather than noisy.

---

## Recommended Service Responsibilities

### Scheduler service
Responsible for:
- checking eligible tracked keywords
- respecting pause status
- avoiding overlap
- dispatching scans

### Scan execution service
Responsible for:
- fetch
- parse
- normalize
- group
- compute snapshot
- build events

### Alert service
Responsible for:
- evaluating alert rules
- deduplicating alert spam
- invoking notifier adapters

### TUI app layer
Responsible for:
- rendering state
- handling user actions
- requesting operations from services

---

## v1 Runtime Risks

### 1. Worker logic becoming tangled with TUI update logic
#### Prevention
Keep services independent from Bubble Tea / TUI concerns.

### 2. In-memory only state becoming too important
#### Prevention
Persist meaningful results and events.

### 3. Overlapping scans creating inconsistent state
#### Prevention
One active scan per keyword.

### 4. App startup feeling blocked or frozen
#### Prevention
Render persisted state first, refresh asynchronously.

### 5. Unbounded retry behavior
#### Prevention
Use interval-based retries, not aggressive retry storms.

---

## Explicit v1 Decisions Recommended

The following should be considered locked for v1 unless later changed intentionally:

1. single local process
2. no separate daemon in v1
3. one active scan at a time per keyword
4. next run based on completion time
5. persisted state as source of truth for last known results
6. TUI is consumer/controller, not scanner
7. scan failures are isolated and non-fatal to whole app

---

## Suggested Future Evolution Path

### v1
Single process local app with embedded scheduler and worker.

### v2
Optional headless mode or worker-only mode.

### v3
Separate API server / worker process if web interface becomes primary.

### Why this path works
It avoids over-engineering now while keeping the architecture movable later.

---

## Recommended Next Documents

After worker runtime, the next useful documents are:

1. `docs/alert-strategy-v1.md`
2. `docs/database-schema-v1.md`
3. `docs/project-structure-v1.md`
4. `docs/scraping-feasibility-notes.md`

---

## Final Summary

The best v1 runtime model is a single-process local application with a clear internal separation between:
- TUI
- scheduler
- scan execution
- alert dispatch
- persistence

This gives the project:
- simpler implementation
- lower operational complexity
- better local development speed
- enough structure to evolve later

The most important runtime rule for trust and stability is:

> never allow overlapping scans for the same tracked keyword

If that rule is preserved, and if persisted state remains the basis for recovery and rendering, the runtime will stay much easier to reason about.
