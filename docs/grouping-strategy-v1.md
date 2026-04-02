# Grouping Strategy v1

## Purpose

This document defines the initial grouping and deduplication strategy for PriceAlert / DealHunt v1.

The purpose of grouping is to turn noisy marketplace listings into a more useful market view.

Without grouping, keyword search results can be too noisy because they may contain:

- duplicate-looking listings
- very similar titles from different sellers
- bundle variations
- size variations
- promotional wording that creates false uniqueness

The goal of v1 is **not perfect product identity resolution**.
The goal is to achieve a level of grouping that is:

- predictable
- explainable
- useful for user decision-making
- good enough for a POC without over-engineering

---

## Grouping Goals

### Main goals
- reduce repeated near-identical listing noise
- keep the top deals panel readable
- improve snapshot quality
- improve alert quality
- avoid misleading aggregation from obvious unrelated products

### Non-goals for v1
- perfect SKU matching
- universal product identity across all brands and variations
- machine-learning-based clustering
- marketplace-wide canonical catalog resolution

---

## Core Principle

Grouping in v1 should be:

- conservative enough to avoid merging clearly different products
- practical enough to merge obvious near-duplicates

When uncertain, it is usually safer in v1 to **under-group** than to **over-group**.

Why:
- under-grouping causes extra noise
- over-grouping causes misleading intelligence

Misleading intelligence is more damaging than mild noise.

---

## Grouping Pipeline Overview

The suggested v1 grouping pipeline is:

1. ingest raw listing
2. normalize title
3. extract lightweight tokens and attributes
4. identify disqualifying mismatch signals
5. compute similarity score
6. combine similarity with price sanity check
7. assign to group or create a new group
8. choose representative listing

---

## 1. Raw Input Assumptions

Each raw listing is expected to provide at least:

- title
- seller name
- price
- original price if available
- URL
- source
- scraped timestamp

For grouping, the most important raw fields are:
- title
- price
- seller name

---

## 2. Title Normalization

### Purpose
Normalization reduces superficial differences that should not create separate groups.

### Recommended normalization steps

1. lowercase all text
2. trim outer whitespace
3. collapse repeated spaces
4. remove most punctuation or convert punctuation to spaces
5. normalize common unit formatting
6. remove low-value promo words
7. normalize repeated promotional exaggerations

### Examples

#### Before
- `Minyak Goreng 2L Promo Murah!!!`
- `MINYAK GORENG 2 Liter - Promo`
- `Minyak Goreng 2L Original Ready Stock`

#### After normalization
- `minyak goreng 2l`
- `minyak goreng 2 liter`
- `minyak goreng 2l`

### Important note
Normalization should **not** destroy meaningful product differences.
For example:
- `1l` and `2l` must remain distinct
- `refill` and `botol` may remain distinct if desired
- bundle counts should not be erased blindly

---

## 3. Noise / Stopword Removal

### Purpose
Some words add almost no product identity value and create false differences.

### Candidate low-value terms for v1
Examples only, not exhaustive:

- promo
- murah
- diskon
- original
- ori
- ready stock
- stok tersedia
- gratis ongkir
- termurah
- best seller
- official

### Caution
Do not remove words that may actually affect grouping meaning.
Examples that need caution:
- refill
- botol
- pouch
- pack
- bundle
- pcs
- liter
- kg

### Recommendation
Split stopwords into two categories:

#### A. Safe promotional stopwords
Can usually be removed.

#### B. Product-form words
Should remain unless product rules say otherwise.

---

## 4. Lightweight Attribute Extraction

### Purpose
Before similarity is computed, extract simple product clues from the title.

This makes grouping more explainable and safer.

### Useful attributes for v1
- quantity / size token
  - `1l`
  - `2l`
  - `500ml`
  - `1kg`
  - `2kg`
- bundle token
  - `2x1l`
  - `isi 2`
  - `pack 3`
- packaging/form token
  - `refill`
  - `botol`
  - `pouch`
- key brand token when obvious
  - `bimoli`
  - `filma`
  - `tropical`

### Why this matters
Two listings with high textual overlap may still be meaningfully different if:
- one is `1l`
- one is `2l`

or:
- one is single unit
- one is bundle

---

## 5. Hard Mismatch Rules

Before using soft similarity, the system should apply hard mismatch rules.

If a hard mismatch is detected, the listings should **not** be grouped.

### Recommended v1 hard mismatch rules

#### Size mismatch
Do not group if extracted size tokens clearly differ.
Examples:
- `1l` vs `2l`
- `1kg` vs `2kg`
- `900ml` vs `2l`

#### Bundle mismatch
Do not group if one looks like single unit and one looks like bundle.
Examples:
- `2l` vs `2x1l`
- `1 pcs` vs `pack 3`

#### Strong brand mismatch
Do not group if obvious brand tokens differ.
Examples:
- `bimoli` vs `filma`

#### Strong packaging mismatch
For v1, treat meaningful packaging differences conservatively.
Examples:
- `refill` vs `botol`
- `pouch` vs `bottle`

### Why hard mismatch matters
This protects against the worst error:
merging clearly different products just because most title words overlap.

---

## 6. Similarity Scoring

After normalization and mismatch filtering, compute a lightweight similarity score.

### Recommended v1 approach
Use a simple, explainable string similarity strategy.
Possible approaches:
- token overlap ratio
- Jaccard similarity on normalized tokens
- simple fuzzy title similarity

### Suggested rule of thumb
A pair may be considered groupable when:
- no hard mismatch exists
- title similarity is above a chosen threshold
- price difference is not wildly inconsistent

### Practical recommendation
Start simple and inspect real results.
A likely first threshold range for experimentation:
- around `0.75` to `0.85` token similarity

The exact value should be validated on real samples.

---

## 7. Price Sanity Check

Text similarity alone is not enough.
A price sanity check helps prevent strange merges.

### Purpose
If two listings have nearly identical normalized titles but radically different price levels, they may not represent the same market item.

### Suggested v1 rule
Only allow grouping if price difference is within a reasonable band.

Examples of reasonable logic:
- relative difference within 10–20%
- or absolute difference acceptable only in narrow cases

### Important caution
Price alone must not override strong identity clues.
Price sanity is a secondary check, not primary identity.

---

## 8. Group Key Strategy

### Purpose
Each grouped result should have a stable internal group identity.

### Recommended v1 idea
A group key can be derived from:
- normalized main title tokens
- key brand token if any
- size token if any
- packaging token if any

Example conceptual shape:

```text
brand + core_title + size + package
```

### Important note
The actual `group_key` does not need to be user-friendly.
It only needs to be deterministic and useful internally.

---

## 9. Representative Listing Selection

Each group needs one representative listing for display.

### Recommended v1 rule
Pick the listing with:
1. the lowest valid price
2. if tied, prefer clearer title
3. if still tied, prefer more stable seller naming/order

### Why this works
The top deals panel should show the best actionable listing for the group.

### Additional value
Also keep:
- `listing_count`
- representative seller
- representative title
- sample URL

This helps users understand whether a low price is isolated or supported by multiple similar listings.

---

## 10. Grouping Confidence Mindset

v1 should not pretend grouping is perfect.

A useful internal concept is grouping confidence, even if not shown to users yet.

Possible rough confidence factors:
- exact size match
- exact brand match
- exact packaging match
- high normalized title similarity
- reasonable price closeness

Even if not exposed in UI, this can help future refinement.

---

## 11. Snapshot Semantics Based on Grouping

To improve market summary quality, snapshot metrics should be based on **grouped listings**, not raw listings.

### Recommended v1 semantics
- `min_price`: minimum `best_price` among grouped listings
- `avg_price`: average `best_price` among grouped listings after basic cleaning
- `max_price`: maximum `best_price` among grouped listings after basic cleaning
- `grouped_count`: number of final groups
- `raw_count`: number of raw listings before grouping

### Why
This reduces distortion caused by repeated duplicates and title spam.

---

## 12. Alert Semantics Based on Grouping

Alerts should be driven by grouped results and snapshot state, not raw listing noise.

### Example
If five raw listings are effectively one product group, the system should not generate five separate alerts.

### Recommendation
Alert engine should consume:
- grouped listings
- snapshot values
- recent history

not raw listing stream directly.

---

## 13. Error Modes and Failure Cases

### 13.1 Under-grouping
Same product appears in multiple groups.

#### Effect
- extra noise
- inflated grouped count
- weaker snapshot quality

#### Usually acceptable?
Yes, more acceptable than over-grouping in v1.

---

### 13.2 Over-grouping
Different products merged into one group.

#### Effect
- misleading top deals
- misleading average price
- misleading alerts
- loss of user trust

#### Severity
High

---

### 13.3 Missing token extraction
Important size or bundle info not detected.

#### Effect
- false grouping
- noisy grouping

#### Recommendation
Keep extraction rules explicit and test them with real examples.

---

### 13.4 Over-aggressive stopword removal
Useful product identity words removed by mistake.

#### Effect
- unrelated products look more similar than they are

#### Recommendation
Treat product-form terms conservatively.

---

### 13.5 Seller-title weirdness
Some sellers may create titles with spam terms, repeated words, emojis, or unusual formatting.

#### Effect
- unstable normalization
- unexpected token overlap behavior

#### Recommendation
Normalization must be robust but not destructive.

---

## 14. v1 Decision Rules Summary

### Safe to group when all are true
- no hard mismatch
- title similarity passes threshold
- price sanity check passes

### Must not group when any are true
- clear size mismatch
- clear bundle mismatch
- clear brand mismatch
- clear packaging mismatch if packaging matters

### Prefer conservative behavior
- if uncertain, create separate groups

---

## 15. Suggested Real-World Test Set

Before locking the implementation, build a manual test set of listing titles such as:

### Should group
- `Minyak Goreng Bimoli 2L Promo`
- `Bimoli Minyak Goreng 2 Liter`
- `Minyak Goreng Bimoli 2L Murah`

### Should not group
- `Bimoli 1L`
- `Bimoli 2L`
- `Bimoli 2x1L`
- `Filma 2L`
- `Bimoli Refill 2L`
- `Bimoli Botol 2L`

This test set will be extremely useful for implementation confidence.

---

## 16. Practical v1 Recommendation

For the first implementation, use this progression:

### Step 1
- normalize title
- extract size and bundle tokens
- remove safe promo stopwords

### Step 2
- apply hard mismatch rules

### Step 3
- compute token-based similarity

### Step 4
- apply price sanity check

### Step 5
- assign group and select cheapest representative

This is simple, explainable, and good enough for a POC.

---

## 17. What Not to Do in v1

Avoid these too early:
- machine learning similarity
- embeddings
- complicated clustering frameworks
- hidden heuristic layers that are hard to reason about
- trying to solve universal product identity

v1 should remain debuggable and explainable.

---

## 18. Recommended Follow-up Documents

After grouping strategy, the next useful documents are:

1. `docs/worker-runtime-v1.md`
2. `docs/alert-strategy-v1.md`
3. `docs/database-schema-v1.md`
4. `docs/scraping-feasibility-notes.md`

---

## Final Summary

Grouping quality is one of the most important trust layers in the product.

The right v1 strategy is not perfect grouping.
It is:
- conservative grouping
- explainable rules
- grouped-based snapshot semantics
- grouped-based alert semantics

If grouping stays predictable and transparent, the rest of the product becomes much easier to trust and improve.
