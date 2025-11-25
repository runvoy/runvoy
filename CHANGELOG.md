# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.3.0] - 2025-11-25

### Added

- [56dcbb7] refactor: split AWS Provider into specialized manager implementations
- [9468bba] fix: add missing perm to event processor
- [d2aedbb] fix(webapp): make API key optional in ConnectionManager for claim flow (#332)
- [1c0ece3] perf(dynamodb): replace Scan with Query for GetExecutionsByRequestID
- [915854f] fix: clean up CloudWatch Logs Insights query formatting (#325)
- [3c6cabc] refactor: implement context-aware logging across all packages (#323)
- [598964e] refactor: move interfaces into contract package for clarity
- [be96490] fix: parse cors env var correctly
- [7243aac] feat: enrich GET /trace endpoint with related resources
- [b22f58c] feat: add SvelteKit routing for shareable URLs in webapp (#313)
- [2570cde] feat: add trace command
- [d7a3a8f] feat: add CloudWatch Logs Insights endpoint for admin request log queries
- [2798571] feat: add backend health check to webapp settings view (#308)
- [f8ac935] feat: add execution list view to webapp
- [d3068c5] refactor: cleanup webapp routing
- [bf55a93] feat: add request ID tracking fields
- [4c59c8e] fix: address wrong deployment flow
- [3353485] refactor: uniform request ID log
- [e55b2b7] fix: webapp: better handle version string
- [7dc417b] feat: standardize view layouts with consistent card styling (#305)
- [ae35bd3] fix: inconsistent layout widths across views (#304)
- [f2106a7] fix: webapp: use join to avoid path mismatch
- [1de8067] refactor: migrate webapp to typescript
- [7e01de7] style: add spinner to infra output
- [7658a83] style: update README badges
- [6597e8d] refactor: simplify execution delete endpoint from /kill to root path
- [758e2f0] Add VERSION to releases

## [v0.2.0] - 2025-11-19

### Added

- Add [CHANGELOG](./CHANGELOG.md)
