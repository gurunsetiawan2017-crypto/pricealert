# GitHub Repo Quality Setup v1

## Purpose

This document records the recommended GitHub-side settings for PriceAlert.

These settings complement the repository files under `.github/` and should be
applied in the GitHub UI for the `main` branch.

---

## Recommended Features To Enable

Enable these repository features:

- Dependency graph
- Dependabot alerts
- Dependabot security updates
- Secret scanning
- Code scanning
- Auto-merge for safe maintenance PRs if desired

For a public repository, most of these are available on GitHub's free tier.

This repository already includes:

- CI workflow
- dependency review workflow
- Dependabot config
- CodeQL workflow

That means GitHub-side activation is mostly about enabling the matching security
features and branch protection.

---

## Recommended Branch Protection / Ruleset For `main`

Apply a ruleset or classic branch protection with these settings:

### Pull request requirements

- require a pull request before merging
- require at least 1 approval
- dismiss stale approvals when new commits are pushed
- require review from Code Owners
- require conversation resolution before merge

### Status checks

Require these checks to pass:

- `test`
- `dependency-review`

Also require the CodeQL check after the first CodeQL workflow run appears.

Typical name:

- `analyze (go)`

### Branch freshness

- require branches to be up to date before merging

### Direct pushes

- restrict direct pushes to `main`

### Optional

- enable auto-merge for small, reviewed dependency PRs
- enable merge queue later if PR volume becomes higher

---

## Why This Is The Recommended Minimum

These controls give the highest value for this repository with low complexity:

- CI catches formatting, module, vet, and test regressions
- dependency review blocks risky dependency changes in PRs
- Dependabot handles dependency/security update discovery
- Code Owners add review pressure to runtime, scraper, scan, and migration changes
- secret scanning and code scanning reduce silent security regressions

---

## Notes

- Repository-side files can enable CI, dependency review, Dependabot behavior,
  and PR templates.
- Branch protection / rulesets still need to be configured in GitHub settings.
- Secret scanning and code scanning availability may depend on repository visibility
  and GitHub plan at the time they are enabled.
- Because this repository now includes an explicit `codeql.yml` workflow, you do
  not need to use "default setup". Enabling code scanning support in GitHub UI
  is enough for the workflow to publish results.

---

## Click-by-Click Activation

### 1. Enable Dependency Graph

1. Open the repository on GitHub
2. Click `Settings`
3. Click `Security`
4. In `Dependency graph`, click `Enable`

### 2. Enable Dependabot

1. Stay in `Settings` -> `Security`
2. Enable `Dependabot alerts`
3. Enable `Dependabot security updates`

### 3. Enable Secret Scanning

1. Stay in `Settings` -> `Security`
2. Find `Secret scanning`
3. Click `Enable`

### 4. Enable Code Scanning Support

1. Stay in `Settings` -> `Security`
2. Open `Code scanning`
3. If GitHub asks to enable code scanning, enable it
4. Do not choose default setup, because this repository already has a
   `.github/workflows/codeql.yml` workflow
5. After the next workflow run, confirm that a CodeQL check appears in PR checks

### 5. Enable Actions If Prompted

1. Open the `Actions` tab
2. If GitHub asks whether workflows should be enabled, confirm
3. Wait for these workflows to appear:
   - `CI`
   - `Dependency Review`
   - `CodeQL`

### 6. Add Branch Protection / Ruleset For `main`

1. Open `Settings`
2. Open `Rules`
3. Click `New ruleset`
4. Choose `New branch ruleset`
5. Name it something like `main protection`
6. Target branch: `main`

Enable:

- require a pull request before merging
- require at least 1 approval
- dismiss stale approvals
- require review from Code Owners
- require conversation resolution
- require status checks to pass
- require branches to be up to date before merging
- block force pushes

Required checks:

- `test`
- `dependency-review`
- `analyze (go)` once it appears
