# Alert Strategy v1

## Purpose

This document defines the initial alert strategy for PriceAlert / DealHunt v1.

The alert system is one of the main trust layers of the product.
Even if scraping and grouping work correctly, the product can still fail if alerts are:

- too frequent
- repetitive
- misleading
- too aggressive in wording
- triggered by noisy or low-confidence data

The purpose of this strategy is to make alerts:

- useful
- conservative enough for v1
- explainable
- resistant to spam
- aligned with grouped snapshot semantics

---

## High-Level Principle

Alerts in v1 should be driven by:

- grouped listings
- market snapshot state
- recent history

Alerts should **not** be driven directly by raw listing noise.

This is critical because raw listings can contain:
- duplicates
- spammy title variants
- bundle mismatches
- ranking fluctuations

---

## Core Alert Goals

### Main goals
- notify the user when something materially interesting happens
- reduce the need for constant manual checking
- highlight potentially strong buying opportunities
- keep logs and Telegram messages meaningful

### Non-goals for v1
- predicting future market movement
- perfect buy-timing intelligence
- highly personalized buying advice
- advanced behavioral scoring

---

## Alert Philosophy for v1

### 1. Prefer fewer, better alerts
A missed minor opportunity is less harmful than a noisy product that users stop trusting.

### 2. Alerts should be stateful, not purely stateless
The system should remember whether something similar was already alerted recently.

### 3. Alerts should reflect grouped market state
A single low raw listing should not automatically become a strong alert unless it survives grouping/snapshot logic.

### 4. Signal wording should be conservative
Especially in v1, do not overstate certainty.

---

## Recommended v1 Alert Types

### 1. Threshold Hit
Trigger when current grouped market minimum is below the user threshold.

### 2. New Lowest
Trigger when the current grouped market minimum is the lowest observed value in the recent historical window.

### 3. Significant Price Drop
Optional in v1, but acceptable if implemented conservatively.
Trigger when the current grouped market minimum drops meaningfully relative to recent baseline.

### 4. Runtime / Delivery Events
These are not buying alerts, but they may appear in activity logs.
Examples:
- scan failed
- Telegram failed
- keyword paused
- keyword resumed

---

## Recommended Alert Scope

### User-facing buying alerts
These should be the main Telegram-worthy alerts:
- threshold_hit
- new_lowest
- price_drop

### Operational events
These should primarily stay in the activity log:
- scan_started
- scan_completed
- scan_failed
- telegram_failed
- keyword_paused
- keyword_resumed

### Recommendation
Do not send every operational event to Telegram in v1.
Telegram should stay focused on actionable alerts.

---

## Signal Naming Strategy

### Problem
A label like `BUY_NOW` sounds strong and final.
That may be too aggressive for early versions.

### Recommended v1 approach
Keep machine-level signal enums if desired, but use conservative user-facing wording.

#### Internal signal enums
- `BUY_NOW`
- `GOOD_DEAL`
- `NORMAL`
- `NO_DATA`

#### Recommended user-facing wording
- `Strong deal detected`
- `Good deal detected`
- `Market looks normal`
- `No usable data yet`

### Alternative safer enum option
If desired, the system can rename machine-level enums later to:
- `STRONG_DEAL`
- `GOOD_DEAL`
- `NORMAL`
- `NO_DATA`

This may be safer semantically.

---

## Alert Trigger Semantics

## 1. Threshold Hit

### Meaning
The current grouped minimum price is at or below the configured threshold.

### Recommended condition
Trigger when:
- tracked keyword has a threshold
- current grouped minimum exists
- current grouped minimum <= threshold

### Important note
Threshold alert should not fire repeatedly every scan while the condition remains true.
It must be anti-spam controlled.

---

## 2. New Lowest

### Meaning
The current grouped minimum is lower than all comparable recent history values in the chosen historical window.

### Recommended v1 historical window
Use a recent bounded window, not all-time history.

Examples:
- last 24 hours
- last 7 days

### Recommendation
For v1, keep the window explicit and simple.
A good starting point is:
- compare against recent valid history in last 7 days

### Important note
If there is not enough prior history, this alert should either:
- not fire yet
- or fire with careful labeling such as "lowest observed so far"

Be cautious with early sparse history.

---

## 3. Significant Price Drop

### Meaning
The current grouped minimum has dropped meaningfully compared with recent baseline.

### Recommended baseline
Use recent grouped-history average or median if available.

### Conservative v1 rule
Only support this if:
- enough recent history points exist
- price drop exceeds a meaningful threshold

Example concept:
- at least N recent valid points
- drop >= 10% or 15% from baseline

### Recommendation
This alert can be postponed if it complicates v1 too much.
Threshold hit and new lowest are already strong for a first release.

---

## Anti-Spam Strategy

This is one of the most important sections for v1.

### Main anti-spam goals
- avoid duplicate alerts on each scan
- avoid tiny price movement spam
- avoid multiple alerts for the same market condition
- keep Telegram valuable

### Recommended anti-spam rules

#### Rule 1: Cooldown window
For the same keyword and same alert type, do not re-send within a cooldown period.

Example cooldowns:
- threshold_hit: 30 to 120 minutes
- new_lowest: 30 to 120 minutes
- price_drop: 60 to 180 minutes

Exact values can be tuned later.

#### Rule 2: Meaningful improvement requirement
Do not send repeat alerts unless the price improved meaningfully since the last sent alert.

Example:
- only re-alert if price is lower than last alerted price by at least X percent
- or by at least a minimum absolute delta

#### Rule 3: Condition transition awareness
Prefer sending alerts when entering an interesting state, not while merely staying in it.

Examples:
- price crosses below threshold -> alert
- price remains below threshold for next 6 scans -> do not alert repeatedly

#### Rule 4: One grouped market state, not many raw triggers
Do not allow multiple raw listings in the same group to trigger multiple equivalent alerts.

---

## Recommended v1 Anti-Spam State

For each tracked keyword and alert type, it is useful to track:

- last alerted_at
- last alerted price
- last alerted signal
- last alert event type

This can be stored in:
- AlertEvent history and derived lookup
- or a small dedicated alert state record later

### Recommendation
For v1, deriving from recent AlertEvents may be enough if query logic stays simple.

---

## Alert Confidence Approach

Alerts should be stronger when the grouped market state looks more trustworthy.

### Simple confidence indicators for v1
- grouped listing count is not extremely low
- current price is not wildly isolated from nearby grouped results
- historical comparison is based on enough valid points

### Recommendation
Do not expose formal confidence scoring yet unless needed.
But use confidence ideas internally to avoid over-aggressive alerts.

---

## Telegram Alert Strategy

### Purpose
Telegram is the first external notification channel in v1.

### Design goal
Keep Telegram messages:
- concise
- actionable
- not overly technical
- not overly frequent

### Recommended content structure
A Telegram alert should include:
- keyword
- alert type / meaningful label
- current price
- threshold if relevant
- snapshot context if useful
- representative listing summary
- link if available

### Example structure
- keyword: minyak goreng 2L
- signal: Strong deal detected
- current best price: 23.800
- threshold: 25.000
- note: lowest observed in recent window
- seller: Seller A
- link: representative listing

### Recommendation
Do not include too much raw operational detail in Telegram messages.
That belongs in the TUI activity log.

---

## Log vs Telegram Separation

### Activity log should contain
- scan failures
- scan completions
- keyword lifecycle events
- Telegram delivery failures
- buying alerts

### Telegram should contain
- buying alerts only by default

### Why
This keeps the notification channel from becoming noisy and keeps debugging information inside the app.

---

## Alert Event Semantics

### Recommended event types for buying alerts
- `threshold_hit`
- `new_lowest`
- `price_drop`

### Recommended event types for runtime/ops
- `scan_started`
- `scan_completed`
- `scan_failed`
- `telegram_sent`
- `telegram_failed`
- `keyword_paused`
- `keyword_resumed`

### Recommendation
Keep alert event naming consistent across:
- DB events
- log panel
- notifier payloads

---

## Alert Trigger Order

When a new snapshot is created, the suggested evaluation order is:

1. validate snapshot is usable
2. compare with recent history
3. evaluate threshold condition
4. evaluate new lowest condition
5. evaluate price drop condition
6. apply anti-spam rules
7. emit AlertEvent(s)
8. deliver Telegram if allowed

### Why this order
It keeps evaluation deterministic and makes debugging easier.

---

## Alert Severity vs Display

### Recommendation
Distinguish between:
- machine event type
- log level
- user-facing wording

Example:
- event_type: `new_lowest`
- level: `ALERT`
- user-facing text: `Strong deal detected`

This gives flexibility without muddying semantics.

---

## Edge Cases and Failure Modes

### 1. No prior history
Problem:
A new keyword may not have enough data for meaningful comparisons.

Recommendation:
- threshold alerts can still work
- new-lowest alerts should be careful
- optionally require minimum history count for certain alert types

---

### 2. Very noisy snapshot
Problem:
Grouping may still produce unstable or weak summary quality.

Recommendation:
If grouped result count is too low or data looks suspicious, either:
- suppress strong alerts
- or downgrade message confidence

---

### 3. Alert loop from persistent threshold condition
Problem:
If price stays under threshold for hours, naive logic will spam repeated alerts.

Recommendation:
Use cooldown plus meaningful-improvement requirement.

---

### 4. Telegram send succeeds late or fails intermittently
Problem:
Delivery state may become ambiguous.

Recommendation:
Keep semantics clear:
- alert generation success is separate from Telegram send success
- notifier failures should not invalidate scan success

---

### 5. False new-lowest due to incomplete early history
Problem:
The system may declare a new low too confidently with only a few prior scans.

Recommendation:
Require a minimum amount of history before certain labels become strong.

---

## Recommended v1 Minimum Conditions

### Threshold Hit
Can work immediately if threshold exists.

### New Lowest
Recommended minimum:
- require at least a small number of prior valid history points
- or label carefully as "lowest observed so far"

### Price Drop
Recommended minimum:
- enough recent history points
- clear percentage drop
- optional for first implementation

---

## Suggested Default Alert Policy for v1

If the goal is a conservative but useful v1, the safest default policy is:

### Enable by default
- threshold_hit
- new_lowest

### Optional later or behind stronger condition
- price_drop

### Telegram delivery default
- send only threshold_hit and new_lowest
- do not send runtime operational events

### Anti-spam defaults
- cooldown enabled
- no duplicate same-price alert
- re-alert only on meaningful improvement

---

## Suggested Future Evolution

### v1
- conservative rule-based alerts
- grouped-state based
- simple anti-spam

### v2
- richer confidence signals
- seller behavior context
- smarter trend windows
- configurable alert profiles

### v3
- fake discount-aware alerting
- multi-channel delivery
- more personalized thresholds

---

## Practical Recommendation for Implementation

Start with the smallest trustworthy set:

1. threshold_hit
2. new_lowest
3. cooldown
4. meaningful-improvement check
5. Telegram for actionable alerts only

This is enough to make the product useful without overcomplicating the first implementation.

---

## Suggested Follow-up Documents

After alert strategy, the next most useful documents are:

1. `docs/database-schema-v1.md`
2. `docs/project-structure-v1.md`
3. `docs/scraping-feasibility-notes.md`

---

## Final Summary

The best v1 alert strategy is not the most advanced one.
It is the one users can trust.

That means:
- grouped-state based alerts
- conservative wording
- strong anti-spam behavior
- clear separation between buying alerts and operational events

If alerts remain calm, explainable, and materially useful, they will become one of the strongest features of the product.
