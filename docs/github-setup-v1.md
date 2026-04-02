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
- Code scanning default setup
- Auto-merge for safe maintenance PRs if desired

For a public repository, most of these are available on GitHub's free tier.

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

If CodeQL default setup is enabled, also require its code scanning check.

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
- Secret scanning and CodeQL availability may depend on repository visibility
  and GitHub plan at the time they are enabled.
