# Review: Gaps, Risks, and Failure Modes v1

## Purpose

This document reviews the current product and architecture discussion for PriceAlert / DealHunt v1.

The goal is to identify:

- design gaps that are not fully settled yet
- technical risks that may slow implementation or reduce product quality
- likely error cases and failure modes
- decisions that should be made before coding goes too far

This is not a rejection of the current plan.
The current direction is strong, but this review is meant to reduce avoidable rework.

---

## High-Level Assessment

The overall direction is promising because it already has:

- a clear user value proposition
- a focused initial target user
- a realistic POC scope
- a sensible technical stack
- a good separation between TUI and future JSON/web readiness

However, there are still important gaps and risks.
The biggest ones are not visual or branding problems.
They are mostly around:

- marketplace data quality
- scraping reliability
- grouping accuracy
- alert trustworthiness
- operational behavior of a long-running worker

If these are not handled well, the product can become noisy, misleading, or fragile.

---

## 1. Product Gaps

### 1.1 Core user promise is still stronger than the current POC logic
The intended positioning is:

> help users buy at the right time

But the current POC logic mostly supports:

- seeing current low prices
- triggering threshold alerts
- detecting new lows from recent history

This is useful, but it is not yet the same as true buy-timing intelligence.

#### Risk
Users may expect smarter recommendations than the system can currently justify.

#### Recommendation
For v1 messaging, be careful with claims.
Prefer wording like:

- monitor market prices
- detect cheaper opportunities faster
- surface current strong deals

Avoid overclaiming things like:

- best time to buy
- optimal buy timing
- smart buying intelligence

unless the signal logic is more mature.

---

### 1.2 User segmentation is still slightly mixed
We agreed to focus on UMKM + power users first, which is good.
But the product still carries some assumptions from normal consumer users too.

#### Risk
The UX and feature priorities may become split between:
- operational buyers who want repeatable monitoring
- casual shoppers who want simple promo alerts

These groups can want different things.

#### Recommendation
For v1, explicitly optimize for:
- repeat monitoring
- multiple tracked keywords
- keyboard-first workflow
- tolerating a more technical setup

Treat casual consumer simplicity as a later productization layer.

---

### 1.3 Filtering strategy is still intentionally weak
We already agreed that keyword search is the core, and filtering is basic at first.
That is realistic.
But it also means product accuracy is still fragile.

#### Risk
The user may search for:
- minyak goreng 2L

but results may still include:
- 1L
- 1.8L
- bundles
- unrelated branded variants

#### Recommendation
Define an explicit v1 limitation in docs and UI behavior.
Also decide whether v1 should:
- show noisy but broad results
- or aggressively filter even if recall drops

This tradeoff should be chosen consciously, not implicitly.

---

## 2. Domain and Data Gaps

### 2.1 Product identity is not solved yet
The current design separates raw listings and grouped listings, which is correct.
But the grouping logic still does not have a settled strategy beyond normalization and lightweight grouping.

#### Risk
Two bad outcomes are both possible:
- same product treated as different groups
- different products treated as same group

Both damage trust.

#### Recommendation
Define a documented grouping strategy before implementation.
At minimum, document:
- normalization rules
- stopword removal rules
- acceptable similarity threshold
- how price difference affects grouping
- how size/volume tokens affect grouping

This should probably become its own design document later.

---

### 2.2 Currency assumptions are implicit
Current examples use integer prices, which is good.
But the design still implicitly assumes one currency and one marketplace context.

#### Risk
This is fine for Tokopedia-first, but if currency formatting leaks into domain or contracts, future adaptation becomes harder.

#### Recommendation
Keep prices as integer values only.
If needed later, add a currency field at the snapshot or marketplace/source level, not UI formatting.

---

### 2.3 Snapshot semantics need stronger definition
We have `min_price`, `avg_price`, and `max_price`, but average is ambiguous.

Questions still open:
- average of raw listings?
- average of grouped listings?
- average after filtering?
- average including obvious outliers?

#### Risk
The number may look authoritative but be misleading.

#### Recommendation
Define exact semantics now.
Strong suggestion for v1:
- `min_price`: minimum of grouped listings
- `avg_price`: average of grouped listings after basic cleaning
- `max_price`: maximum of grouped listings after basic cleaning

Avoid mixing raw noisy data into snapshot summary unless explicitly intended.

---

### 2.4 AlertRule may be over-designed for immediate implementation
Option B includes AlertRule, which is future-proof.
But v1 may not need the full generality yet.

#### Risk
Implementation may spend too much effort on general rule systems before the basic product works.

#### Recommendation
Keep AlertRule in the design, but allow v1 implementation to hardcode a small set of supported rules first:
- threshold_below
- new_lowest
- optional price_drop_percent

Do not build a complicated rule engine too early.

---

## 3. Scraping and Source Risks

### 3.1 Tokopedia scraping may be the biggest practical risk
This is probably the single highest real-world implementation risk.

#### Why it matters
Even a great TUI and domain model will fail if the scraper is unreliable.

#### Main risks
- anti-bot measures
- rate limiting
- changing HTML/JSON structure
- hidden APIs changing without notice
- search result inconsistency by location, session, or cookies
- incomplete results due to pagination or lazy loading

#### Recommendation
Before deep UI work, validate scraping feasibility for the specific query flow:
- search by keyword
- collect enough results
- repeat reliably over time
- parse seller, price, promo indicators, and URL consistently

This deserves an early proof-of-feasibility test.

---

### 3.2 Search result determinism may be weaker than expected
Marketplace search rankings may vary.
The same keyword may not always return the same top results in the same order.

#### Risk
User may think the market changed, while actually the search ranking changed.

#### Recommendation
Treat the system as observing sampled marketplace state, not guaranteed full market truth.
Avoid language that suggests perfect completeness.

---

### 3.3 Location and personalization effects are not yet addressed
Marketplace platforms may vary prices, availability, or result ordering based on:
- region
- delivery destination
- login state
- device/session context

#### Risk
The observed results may not match what another user sees.

#### Recommendation
For v1, standardize the scraping context as much as possible.
Document that results reflect the crawler context, not universal market truth.

---

## 4. Worker and Runtime Risks

### 4.1 Background runner behavior is not fully defined yet
We agreed on CLI setup plus service/background worker.
But the runtime model is still not settled.

Open questions include:
- does worker run in the same process as TUI?
- is there a separate daemon?
- what happens when TUI is closed?
- how is restart handled?
- how are crashed jobs recovered?

#### Risk
Architecture confusion and duplicated logic later.

#### Recommendation
Document a clear v1 runtime model soon.
Strong candidate:
- one app process with a worker loop for local single-user mode
- TUI can read persisted state
- do not design a daemon unless needed immediately

Or decide explicitly to build a daemon first. But choose one early.

---

### 4.2 Concurrent scan overlap risk
If scan interval is short and scraping is slow, one scan may overlap the next.

#### Risk
- duplicate work
- DB contention
- duplicate alerts
- stale state races

#### Recommendation
Add a v1 rule:
For a given tracked keyword, never allow overlapping active scans.
If a scan is still running when next interval arrives, either:
- skip the next run
- or queue only one pending rerun

---

### 4.3 Time drift and scheduling jitter are not fully specified
We discussed configurable intervals and hinted at jitter.
But exact scheduling behavior is not yet settled.

#### Risk
- suspiciously robotic scrape timing
- uneven monitoring behavior
- confusing log interpretation

#### Recommendation
Define v1 scheduling behavior clearly:
- interval is approximate
- optional random jitter window
- next run based on completion time or start time, chosen explicitly

I recommend basing next run on completion time for simplicity in v1.

---

## 5. Alert Quality Risks

### 5.1 Alert spam is a major product trust risk
Even if scanning works, alerts can quickly become annoying or ignored.

#### Likely causes
- same threshold hit repeatedly
- tiny price changes generating repeated alerts
- multiple grouped listings generating near-identical events
- repeated Telegram notifications on each scan

#### Recommendation
Define anti-spam rules now.
For example:
- do not re-send identical alert within a cooldown window
- only send if price improved meaningfully from last alerted price
- collapse repeated events into a single stateful alert behavior

This should be treated as a core feature, not a polish item.

---

### 5.2 Signal credibility is still fragile
Signals like `BUY_NOW` are emotionally strong.

#### Risk
If users see weak recommendations labeled `BUY_NOW`, trust can collapse fast.

#### Recommendation
Either:
- use more conservative signals in v1, or
- define strict signal thresholds and document them

A safer v1 option could be:
- `STRONG_DEAL`
- `GOOD_DEAL`
- `NORMAL`
- `NO_DATA`

This is less overconfident than `BUY_NOW`.

---

### 5.3 Telegram delivery semantics are ambiguous
Current event model uses `sent_to_telegram`, but this can mean different things:
- delivery attempted
- delivery succeeded
- delivery accepted by API

#### Risk
Logs become misleading.

#### Recommendation
Clarify semantics.
Better options:
- keep `sent_to_telegram` meaning successful API send only
- or split into fields like `telegram_attempted` and `telegram_sent`

---

## 6. Storage and Data Growth Risks

### 6.1 RawListing volume may grow quickly
Frequent scans plus multiple keywords can create lots of raw listing records.

#### Risk
- MariaDB tables grow fast
- local laptop performance degrades
- history queries become slower

#### Recommendation
Plan retention early.
Possible v1 policy:
- keep full RawListing only for recent window
- keep grouped results and snapshots longer
- archive or prune raw data after a defined period

---

### 6.2 Indexing strategy is not defined yet
The design is good conceptually, but DB performance will depend heavily on indexes.

#### Risk
As soon as history grows, dashboard and detail screens may slow down.

#### Recommendation
When database schema is written, ensure indexes for at least:
- tracked_keyword_id
- scan_job_id
- created_at / snapshot_at / recorded_at
- status where relevant

---

### 6.3 ID strategy is not defined yet
The domain uses string IDs in docs, but generation strategy is not yet decided.

#### Risk
Inconsistent ID choices can complicate repositories and contracts later.

#### Recommendation
Choose early between:
- auto-increment ints internally with string mapping in DTOs
- UUID/ULID style IDs everywhere

For local-first single-user Go app, ULID or UUID can be clean and future-friendly.

---

## 7. TUI / UX Risks

### 7.1 TUI may become data-rich but action-poor
The current wireframe is clear, but there is a subtle risk.
The app may show a lot of information without making decisions easier enough.

#### Risk
User sees many numbers but still does not know what to do.

#### Recommendation
Always prioritize action-supporting output:
- current best price
- how it compares to recent history
- whether this is notable enough to care

Do not overload the screen with metrics that do not change behavior.

---

### 7.2 Keyboard-first UX may hide discoverability issues
Power users like keyboard-driven interfaces, but new users may not discover shortcuts easily.

#### Recommendation
Keep shortcut hints visible at all times in the main screen.
Also consider a lightweight help modal in v1.1 if needed.

---

### 7.3 Empty, loading, and error states need equal care
The happy path UI is well discussed, but fragile systems are judged most during failure states.

#### Recommendation
Design these states explicitly in implementation:
- no keyword yet
- no results yet
- scan failed
- Telegram not configured
- stale data

---

## 8. Future Web Interface Risks

### 8.1 TUI-first design may still leak too much into internal structure
Even though we agreed to stay JSON-ready, implementation could still accidentally become TUI-shaped.

#### Risk
Web layer later becomes awkward because core services are too tied to TUI flows.

#### Recommendation
Keep a strict separation:
- domain/application services produce DTOs
- TUI is only one consumer
- web API later becomes another consumer

---

### 8.2 API contracts may become unstable if not treated seriously
We now have JSON contracts documented, which is good.
But if implementation changes them casually, later web work becomes messy.

#### Recommendation
Treat key DTOs as semi-stable contracts once coding begins:
- DashboardState
- KeywordDetail
- MarketSnapshot
- AlertEvent

---

## 9. Operational and Legal/Product Risks

### 9.1 Terms-of-service or access policy risk
Depending on implementation approach, scraping may run into platform policy issues.

#### Risk
- blocking
- account/session restrictions
- instability over time

#### Recommendation
Be conservative in request rate, avoid aggressive behavior, and validate risk tolerance early.
This is more a product/operational concern than a code concern, but it matters.

---

### 9.2 User trust risk from imperfect data
If grouped results are occasionally wrong, or price state changes too quickly, users may perceive the whole product as unreliable.

#### Recommendation
Prefer honest wording in UI:
- observed listings
- latest scan
- sampled results

Avoid claims that imply perfect market completeness.

---

## 10. Recommended Pre-Implementation Decisions

Before implementation goes too far, these decisions should be locked:

1. exact runtime model
   - single process vs daemon-like behavior

2. exact snapshot semantics
   - how min/avg/max are computed

3. exact grouping logic v1
   - normalization, similarity, size token handling

4. exact alert anti-spam rules
   - cooldown, meaningful price delta, dedup behavior

5. exact scan scheduling behavior
   - overlap policy, jitter, next-run calculation

6. exact Telegram delivery semantics
   - attempted vs succeeded

7. exact retention strategy
   - raw vs grouped vs snapshot history

---

## 11. Recommended Risk Priority

### Highest priority
These should be validated first because they can break the whole product:
- Tokopedia scraping feasibility
- grouping accuracy quality
- alert spam control
- runtime/worker model clarity

### Medium priority
These matter soon after the first scans work:
- DB retention strategy
- snapshot semantics
- signal naming confidence
- index strategy

### Lower priority for now
These can wait slightly longer:
- advanced rule engine flexibility
- web API versioning concerns
- richer notifier abstraction

---

## 12. Suggested Next Documents

To reduce the biggest risks, the next documents that would be most useful are:

1. `docs/grouping-strategy-v1.md`
2. `docs/worker-runtime-v1.md`
3. `docs/database-schema-v1.md`
4. `docs/alert-strategy-v1.md`
5. `docs/scraping-feasibility-notes.md`

---

## Final Review Summary

The project direction is good and worth continuing.
The biggest danger is not lack of ideas.
The biggest danger is building a polished shell around unreliable marketplace observations.

If the system can make these parts trustworthy enough:
- scraping
- grouping
- snapshot semantics
- alert quality

then the TUI and product identity can become genuinely strong.

If not, the experience may look impressive but feel unreliable.

So the strongest next move is not more UI detail.
It is reducing uncertainty in the data pipeline and runtime behavior.
